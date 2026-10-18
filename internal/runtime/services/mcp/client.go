package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultCallTimeout = 2 * time.Minute
	maxDiscoveredTools = 10000
)

type Client struct {
	transport Transport
	protocol  *sdk.Client
	mu        sync.Mutex
	session   *sdk.ClientSession
	closed    bool
	progress  map[string]func(ProgressUpdate)
	nextToken atomic.Uint64
}

func NewClient(t Transport) *Client {
	c := &Client{transport: t, progress: make(map[string]func(ProgressUpdate))}
	c.protocol = sdk.NewClient(&sdk.Implementation{Name: "agent-harness", Version: "development"}, &sdk.ClientOptions{
		Capabilities:                &sdk.ClientCapabilities{},
		MultiRoundTrip:              &sdk.MultiRoundTripOptions{Disabled: true},
		ProgressNotificationHandler: c.reportProgress,
	})
	return c
}

// Initialize starts the subprocess, negotiates the protocol and sends the
// initialized notification. The setup context does not own the live session.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	session, err := c.protocol.Connect(ctx, c.transport, &sdk.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.closed || c.session != nil {
		c.mu.Unlock()
		_ = session.Close()
		return nil, errors.New("MCP client is closed or already initialized")
	}
	c.session = session
	c.mu.Unlock()
	return session.InitializeResult(), nil
}

func (c *Client) activeSession() (*sdk.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.session == nil {
		return nil, errors.New("MCP client is not connected")
	}
	return c.session, nil
}

// ListTools follows the entire catalog, rejecting ambiguous names and cursor
// loops instead of advertising an incomplete or nonterminating tool list.
func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error) {
	session, err := c.activeSession()
	if err != nil {
		return nil, err
	}
	var tools []ToolDef
	seenNames, seenCursors := map[string]bool{}, map[string]bool{}
	cursor := ""
	for {
		page, err := session.ListTools(ctx, &sdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, tool := range page.Tools {
			if tool.Name == "" || seenNames[tool.Name] {
				return nil, fmt.Errorf("MCP catalog has an empty or duplicate tool name %q", tool.Name)
			}
			if len(tools) >= maxDiscoveredTools {
				return nil, fmt.Errorf("MCP catalog exceeds %d tools", maxDiscoveredTools)
			}
			schema, ok := tool.InputSchema.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("MCP tool %q has no object input schema", tool.Name)
			}
			output, _ := tool.OutputSchema.(map[string]any)
			tools = append(tools, ToolDef{Name: tool.Name, Description: tool.Description,
				InputSchema: schema, OutputSchema: output, Annotations: tool.Annotations})
			seenNames[tool.Name] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			return tools, nil
		}
		if seenCursors[cursor] {
			return nil, fmt.Errorf("MCP catalog repeated pagination cursor %q", cursor)
		}
		seenCursors[cursor] = true
	}
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	return c.callTool(ctx, name, args, nil)
}

func (c *Client) callTool(ctx context.Context, name string, args map[string]any, onProgress func(ProgressUpdate)) (*CallToolResult, error) {
	session, err := c.activeSession()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()
	}
	params := &sdk.CallToolParams{Name: name, Arguments: args}
	if onProgress != nil {
		token := fmt.Sprintf("agent-harness-%d", c.nextToken.Add(1))
		params.SetProgressToken(token)
		c.mu.Lock()
		c.progress[token] = onProgress
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			delete(c.progress, token)
			c.mu.Unlock()
		}()
	}
	return session.CallTool(ctx, params)
}

func (c *Client) reportProgress(_ context.Context, req *sdk.ProgressNotificationClientRequest) {
	token, ok := req.Params.ProgressToken.(string)
	if !ok {
		return
	}
	c.mu.Lock()
	callback := c.progress[token]
	c.mu.Unlock()
	if callback != nil {
		callback(ProgressUpdate{Message: req.Params.Message, Progress: req.Params.Progress, Total: req.Params.Total})
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	session := c.session
	c.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (c *Client) Wait() error {
	session, err := c.activeSession()
	if err != nil {
		return err
	}
	return session.Wait()
}
