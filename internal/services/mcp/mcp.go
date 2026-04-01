package mcp

import (
	"context"
	"fmt"
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

// Manager handles MCP server lifecycle.
type Manager struct {
	connections map[string]*Connection
}

// NewManager creates an MCP manager.
func NewManager() *Manager {
	return &Manager{connections: make(map[string]*Connection)}
}

// Connect establishes a connection to an MCP server.
// This is a stub implementation showing the pattern.
func (m *Manager) Connect(ctx context.Context, name, transportType string, config map[string]any) (*Connection, error) {
	conn := &Connection{
		Name:   name,
		Type:   transportType,
		Status: "pending",
	}

	switch transportType {
	case "stdio":
		// Pattern: spawn child process, communicate over stdin/stdout
		conn.Status = "connected"
	case "sse":
		// Pattern: HTTP EventSource connection
		conn.Status = "connected"
	case "http":
		// Pattern: Streamable HTTP
		conn.Status = "connected"
	case "ws":
		// Pattern: WebSocket
		conn.Status = "connected"
	default:
		return nil, fmt.Errorf("unsupported MCP transport: %s", transportType)
	}

	m.connections[name] = conn
	return conn, nil
}

// Disconnect closes a connection.
func (m *Manager) Disconnect(name string) error {
	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("no such MCP connection: %s", name)
	}
	conn.Status = "disconnected"
	delete(m.connections, name)
	return nil
}

// ListConnections returns all active connections.
func (m *Manager) ListConnections() []*Connection {
	out := make([]*Connection, 0, len(m.connections))
	for _, c := range m.connections {
		out = append(out, c)
	}
	return out
}

// GetConnection retrieves a single connection.
func (m *Manager) GetConnection(name string) (*Connection, bool) {
	c, ok := m.connections[name]
	return c, ok
}

// CallTool invokes a tool on an MCP server.
// This is the pattern used by the MCPTool wrapper.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error) {
	conn, ok := m.connections[serverName]
	if !ok {
		return nil, fmt.Errorf("MCP server not connected: %s", serverName)
	}
	if conn.Status != "connected" {
		return nil, fmt.Errorf("MCP server not ready: %s (%s)", serverName, conn.Status)
	}

	// In a full implementation, this would serialize the request
	// and send it over the transport.
	return fmt.Sprintf("MCP tool %s/%s called with %+v", serverName, toolName, args), nil
}
