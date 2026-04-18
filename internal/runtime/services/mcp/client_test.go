package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type mockTransport struct {
	responses []*JSONRPCResponse
	index     int
}

func (m *mockTransport) Start(ctx context.Context) error { return nil }
func (m *mockTransport) Close() error                     { return nil }
func (m *mockTransport) Send(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	if m.index >= len(m.responses) {
		return nil, fmt.Errorf("no more responses")
	}
	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func TestClientInitialize(t *testing.T) {
	result, _ := json.Marshal(InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo:      Info{Name: "test-server", Version: "1.0"},
	})
	mt := &mockTransport{responses: []*JSONRPCResponse{{ID: 1, Result: result}}}
	client := NewClient(mt)

	res, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if res.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", res.ProtocolVersion)
	}
	if res.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name test-server, got %s", res.ServerInfo.Name)
	}
}

func TestClientListTools(t *testing.T) {
	result, _ := json.Marshal(ListToolsResult{Tools: []ToolDef{
		{Name: "echo", Description: "echo input", InputSchema: map[string]any{"type": "object"}},
	}})
	mt := &mockTransport{responses: []*JSONRPCResponse{{ID: 1, Result: result}}}
	client := NewClient(mt)

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("list tools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Errorf("expected 1 tool named echo, got %+v", tools)
	}
}

func TestClientCallTool(t *testing.T) {
	result, _ := json.Marshal(CallToolResult{Content: []ContentItem{{Type: "text", Text: "hello"}}})
	mt := &mockTransport{responses: []*JSONRPCResponse{{ID: 1, Result: result}}}
	client := NewClient(mt)

	res, err := client.CallTool(context.Background(), "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "hello" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestClientJSONRPCError(t *testing.T) {
	mt := &mockTransport{responses: []*JSONRPCResponse{{ID: 1, Error: &JSONRPCError{Code: -32600, Message: "invalid request"}}}}
	client := NewClient(mt)

	_, err := client.Initialize(context.Background())
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response")
	}
}
