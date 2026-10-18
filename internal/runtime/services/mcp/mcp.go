package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Connection is a snapshot, not a mutable handle to manager-owned state.
type Connection struct {
	Name   string
	Type   string
	Tools  []ToolDef
	Status string
}

// ToolDef retains server schemas and advisory annotations. An annotation is
// never an authorization grant: every external tool still asks for approval.
type ToolDef struct {
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	InputSchema  map[string]any       `json:"inputSchema"`
	OutputSchema map[string]any       `json:"outputSchema,omitempty"`
	Annotations  *sdk.ToolAnnotations `json:"annotations,omitempty"`
}

type WrappedToolDef struct {
	ServerName string
	ToolDef
}

type connectionEntry struct {
	conn   Connection
	client *Client
	cancel context.CancelFunc
	ready  chan struct{}
}

// Manager owns server processes. No network or process I/O runs under mu.
type Manager struct {
	mu          sync.RWMutex
	connections map[string]*connectionEntry
	closed      bool
	closeOnce   sync.Once
	closeErr    error
}

func NewManager() *Manager {
	return &Manager{connections: make(map[string]*connectionEntry)}
}

func (m *Manager) Connect(ctx context.Context, name string, cfg McpServerConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	serverType := cfg.Type
	if serverType == "" {
		serverType = "stdio"
	}
	if serverType != "stdio" {
		return fmt.Errorf("unsupported MCP transport: %s", serverType)
	}
	if name == "" || cfg.Command == "" {
		return errors.New("MCP server name and command must be nonempty")
	}
	entry := &connectionEntry{conn: Connection{Name: name, Type: serverType, Status: "pending"}, cancel: cancel, ready: make(chan struct{})}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("MCP manager is closed")
	}
	if _, exists := m.connections[name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("MCP connection %s already exists", name)
	}
	m.connections[name] = entry
	m.mu.Unlock()
	defer close(entry.ready)

	client := NewClient(NewStdioTransport(cfg.Command, cfg.Args, cfg.Env))
	connected := false
	defer func() {
		if !connected {
			_ = client.Close()
			m.mu.Lock()
			if m.connections[name] == entry {
				delete(m.connections, name)
			}
			m.mu.Unlock()
		}
	}()
	if _, err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize MCP server %s: %w", name, err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools for MCP server %s: %w", name, err)
	}
	m.mu.Lock()
	if m.closed || m.connections[name] != entry || ctx.Err() != nil {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %s connection was cancelled", name)
	}
	entry.client = client
	entry.conn.Tools = tools
	entry.conn.Status = "connected"
	connected = true
	m.mu.Unlock()
	go m.watchConnection(name, entry)
	return nil
}

func (m *Manager) watchConnection(name string, entry *connectionEntry) {
	_ = entry.client.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connections[name] == entry {
		entry.conn.Status = "disconnected"
	}
}

// LoadAndConnect attempts every configured server. A failed extension must
// not hide tools belonging to an independent, healthy server.
func (m *Manager) LoadAndConnect(ctx context.Context, servers map[string]McpServerConfig) error {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var failures []error
	for _, name := range names {
		if err := m.Connect(ctx, name, servers[name]); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	entry, ok := m.connections[name]
	delete(m.connections, name)
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no such MCP connection: %s", name)
	}
	return closeEntry(entry)
}

func closeEntry(entry *connectionEntry) error {
	entry.cancel()
	<-entry.ready
	if entry.client != nil {
		return entry.client.Close()
	}
	return nil
}

// Close is idempotent, cancels pending setups, and reaps every owned process.
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		entries := m.connections
		m.connections = make(map[string]*connectionEntry)
		m.mu.Unlock()
		var failures []error
		for _, entry := range entries {
			if err := closeEntry(entry); err != nil {
				failures = append(failures, err)
			}
		}
		m.closeErr = errors.Join(failures...)
	})
	return m.closeErr
}

func connectionSnapshot(entry *connectionEntry) *Connection {
	conn := entry.conn
	conn.Tools = append([]ToolDef(nil), conn.Tools...)
	return &conn
}

func (m *Manager) ListConnections() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Connection, 0, len(m.connections))
	for _, entry := range m.connections {
		out = append(out, connectionSnapshot(entry))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m *Manager) GetConnection(name string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.connections[name]
	if !ok {
		return nil, false
	}
	return connectionSnapshot(entry), true
}

// CallTool is the text-facing compatibility entry point. The wrapper uses
// CallToolWithProgress to preserve error status alongside budgeted content.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
	result, err := m.CallToolWithProgress(ctx, serverName, toolName, args, nil)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return nil, fmt.Errorf("MCP tool error: %s", result.Content)
	}
	return result.Content, nil
}

func (m *Manager) CallToolWithProgress(ctx context.Context, serverName, toolName string, args map[string]any, onProgress func(ProgressUpdate)) (Result, error) {
	m.mu.RLock()
	entry, ok := m.connections[serverName]
	if !ok || entry.conn.Status != "connected" {
		m.mu.RUnlock()
		return Result{}, fmt.Errorf("MCP server not connected: %s", serverName)
	}
	client := entry.client
	m.mu.RUnlock()
	result, err := client.callTool(ctx, toolName, args, onProgress)
	if err != nil {
		return Result{}, fmt.Errorf("MCP %s/%s: %w", serverName, toolName, err)
	}
	return renderToolResult(result)
}

func (m *Manager) AllToolDefs() []WrappedToolDef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]WrappedToolDef, 0)
	for name, entry := range m.connections {
		if entry.conn.Status != "connected" {
			continue
		}
		for _, tool := range entry.conn.Tools {
			out = append(out, WrappedToolDef{ServerName: name, ToolDef: tool})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerName != out[j].ServerName {
			return out[i].ServerName < out[j].ServerName
		}
		return out[i].Name < out[j].Name
	})
	return out
}
