// Package mcppeer supplies an independent, raw JSON-RPC subprocess for MCP
// acceptance tests. It deliberately does not use the production client's SDK.
package mcppeer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type request struct {
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peer struct {
	out         io.Writer
	mu          sync.Mutex
	initialized bool
	mode        string
	workers     sync.WaitGroup
}

// RunIfRequested is called by TestMCPPeerProcess in each test binary that
// needs a peer. Only a child selected with the private fixture flag serves.
func RunIfRequested() {
	mode := os.Getenv("AH_MCP_TEST_PEER")
	if mode == "" {
		return
	}
	err := Serve(os.Stdin, os.Stdout, mode)
	if path := os.Getenv("AH_MCP_TEST_EXIT"); path != "" {
		_ = os.WriteFile(path, []byte("exited"), 0o600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Command selects the fixture entry point in the calling package's binary.
func Command() (string, []string, error) {
	exe, err := os.Executable()
	return exe, []string{"-test.run=^TestMCPPeerProcess$"}, err
}

// Serve accepts newline-delimited JSON-RPC, enforcing initialization and
// injecting notifications, pagination, tool failures, and late responses.
func Serve(in io.Reader, out io.Writer, mode string) error {
	p := &peer{out: out, mode: mode}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return err
		}
		if path := os.Getenv("AH_MCP_TEST_TRACE"); path != "" {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, _ = f.Write(append(append([]byte{}, scanner.Bytes()...), '\n'))
			_ = f.Close()
		}
		switch req.Method {
		case "initialize":
			if mode == "hang-initialize" {
				continue
			}
			if mode == "reject-initialize" {
				p.failure(req.ID, -32602, "fixture rejects initialization")
				continue
			}
			p.result(req.ID, map[string]any{
				"protocolVersion": "2025-11-25",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "raw-fixture", "version": "1.0.0"},
			})
		case "notifications/initialized":
			p.initialized = true
		case "notifications/cancelled":
			// Some peers ignore cancellation; still deliver the original
			// late reply to exercise response correlation after cancellation.
		case "ping":
			p.result(req.ID, map[string]any{})
		case "tools/list":
			if !p.initialized {
				p.failure(req.ID, -32000, "initialized notification required")
				continue
			}
			var params struct {
				Cursor string `json:"cursor"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if params.Cursor == "" {
				p.notify("notifications/message", map[string]any{"level": "info", "data": "catalog loading"})
				p.result(req.ID, map[string]any{"tools": []any{definition("inspect")}, "nextCursor": "second-page"})
			} else {
				page := map[string]any{"tools": []any{definition("echo"), definition("tool_error"), definition("slow"), definition("exit")}}
				if mode == "duplicate-catalog" {
					page["tools"] = []any{definition("inspect")}
				}
				if mode == "cursor-loop" {
					page["nextCursor"] = "second-page"
				}
				p.result(req.ID, page)
			}
		case "tools/call":
			if mode == "exit-on-call" {
				return nil
			}
			p.call(req)
		default:
			if req.ID != nil {
				p.failure(req.ID, -32601, "method not supported by fixture")
			}
		}
	}
	p.workers.Wait()
	return scanner.Err()
}

func definition(name string) map[string]any {
	return map[string]any{"name": name, "description": "fixture " + name,
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		"annotations": map[string]any{"readOnlyHint": true}}
}

func (p *peer) call(req request) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
		Meta      struct {
			ProgressToken any `json:"progressToken"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		p.failure(req.ID, -32602, err.Error())
		return
	}
	if params.Meta.ProgressToken != nil {
		p.notify("notifications/progress", map[string]any{
			"progressToken": params.Meta.ProgressToken, "progress": 1, "message": "Inspecting fixture artifact",
		})
	}
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": "first"}, map[string]any{"type": "text", "text": "second"}}}
	switch params.Name {
	case "inspect", "tool_error":
		value := map[string]any{"evidence_id": "ev_fixture", "observations": []string{"verified source"}, "limitations": []string{"static inspection only"}}
		if params.Name == "tool_error" {
			value = map[string]any{"error": map[string]any{"code": "TARGET_MISSING", "remediation": "supply a readable target"}}
			result["isError"] = true
		}
		if text, ok := params.Arguments["padding"].(string); ok {
			value["padding"] = text
		}
		text, _ := json.Marshal(value)
		result["content"] = []any{map[string]any{"type": "text", "text": string(text)}}
		result["structuredContent"] = value
		result["_meta"] = map[string]any{"provenance": map[string]any{"untrusted_content": true, "source": "analysis_target"}}
	case "slow":
		p.workers.Add(1)
		go func() {
			defer p.workers.Done()
			time.Sleep(150 * time.Millisecond)
			p.result(req.ID, map[string]any{"content": []any{map[string]any{"type": "text", "text": "late original response"}}})
		}()
		return
	case "echo":
		if p.mode == "late-response" {
			time.Sleep(220 * time.Millisecond)
		}
	case "large":
		result["content"] = []any{map[string]any{"type": "text", "text": strings.Repeat("evidence", 16000)}}
	}
	p.result(req.ID, result)
}

func (p *peer) result(id, result any) {
	p.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func (p *peer) failure(id any, code int, message string) {
	p.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}
func (p *peer) notify(method string, params any) {
	p.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (p *peer) write(value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = json.NewEncoder(p.out).Encode(value)
}
