package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Connection represents an active MCP server connection.
type Connection struct {
	Name   string
	Type   string // stdio, sse, http, ws
	Tools  []ToolDef
	Status string // connected, pending, error
}

// ToolDef describes a tool exposed by an MCP server.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// WrappedToolDef tags a ToolDef with its originating server name.
type WrappedToolDef struct {
	ServerName string
	ToolDef
}

// connectionEntry holds the runtime state for a single MCP connection.
type connectionEntry struct {
	conn   *Connection
	client *Client
	tools  []ToolDef
}

// Manager handles MCP server lifecycle.
type Manager struct {
	mu          sync.RWMutex
	connections map[string]*connectionEntry
}

// NewManager creates an MCP manager.
func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]*connectionEntry),
	}
}

// Connect establishes a connection to an MCP server using the provided config.
func (m *Manager) Connect(ctx context.Context, name string, cfg McpServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[name]; exists {
		return fmt.Errorf("MCP connection %s already exists", name)
	}

	serverType := cfg.Type
	if serverType == "" {
		serverType = "stdio"
	}

	var transport Transport
	switch serverType {
	case "stdio":
		transport = NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	default:
		return fmt.Errorf("unsupported MCP transport: %s", serverType)
	}

	client := NewClient(transport)

	if _, err := client.Initialize(ctx); err != nil {
		_ = transport.Close()
		return fmt.Errorf("failed to initialize MCP server %s: %w", name, err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		_ = transport.Close()
		return fmt.Errorf("failed to list tools for MCP server %s: %w", name, err)
	}

	conn := &Connection{
		Name:   name,
		Type:   serverType,
		Tools:  tools,
		Status: "connected",
	}

	m.connections[name] = &connectionEntry{
		conn:   conn,
		client: client,
		tools:  tools,
	}

	return nil
}

// LoadAndConnect connects to all servers in the provided map.
func (m *Manager) LoadAndConnect(ctx context.Context, servers map[string]McpServerConfig) error {
	for name, cfg := range servers {
		if err := m.Connect(ctx, name, cfg); err != nil {
			return err
		}
	}
	return nil
}

// Disconnect closes a connection.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("no such MCP connection: %s", name)
	}

	if entry.client != nil {
		_ = entry.client.Close()
	}
	entry.conn.Status = "disconnected"
	delete(m.connections, name)
	return nil
}

// ListConnections returns all active connections.
func (m *Manager) ListConnections() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Connection, 0, len(m.connections))
	for _, entry := range m.connections {
		out = append(out, entry.conn)
	}
	return out
}

// GetConnection retrieves a single connection.
func (m *Manager) GetConnection(name string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.connections[name]
	if !ok {
		return nil, false
	}
	return entry.conn, true
}

// CallTool invokes a tool on an MCP server.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
	m.mu.RLock()
	entry, ok := m.connections[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("MCP server not connected: %s", serverName)
	}
	if entry.conn.Status != "connected" {
		return nil, fmt.Errorf("MCP server not ready: %s (%s)", serverName, entry.conn.Status)
	}

	result, err := entry.client.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, err
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	text := strings.Join(texts, "")

	if result.IsError {
		return nil, fmt.Errorf("mcp tool error: %s", text)
	}

	return text, nil
}

// AllToolDefs returns all discovered tools tagged with their server name.
func (m *Manager) AllToolDefs() []WrappedToolDef {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]WrappedToolDef, 0)
	for name, entry := range m.connections {
		for _, tool := range entry.tools {
			out = append(out, WrappedToolDef{
				ServerName: name,
				ToolDef:    tool,
			})
		}
	}
	return out
}
