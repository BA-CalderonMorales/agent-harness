package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func connectedPeer(t *testing.T, mode string) *Manager {
	t.Helper()
	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.Connect(ctx, "fixture", peerConfig(t, mode)); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestClientInitializeCompletesWireHandshake(t *testing.T) {
	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	cfg := peerConfig(t, "normal")
	trace := filepath.Join(t.TempDir(), "requests.jsonl")
	cfg.Env["AH_MCP_TEST_TRACE"] = trace
	if err := m.Connect(context.Background(), "fixture", cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"initialize", "notifications/initialized", "tools/list", "tools/list"}
	if len(lines) != len(want) {
		t.Fatalf("unexpected wire transcript: %s", data)
	}
	for i, line := range lines {
		var request map[string]any
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			t.Fatal(err)
		}
		if request["method"] != want[i] {
			t.Fatalf("request %d = %v, want %s", i, request["method"], want[i])
		}
		if i == 1 && request["id"] != nil {
			t.Fatal("initialized was sent as a request instead of a notification")
		}
		if i == 0 {
			caps := request["params"].(map[string]any)["capabilities"].(map[string]any)
			if caps["roots"] != nil || caps["sampling"] != nil || caps["elicitation"] != nil {
				t.Fatalf("advertised unsupported capabilities: %v", caps)
			}
		}
	}
}

func TestClientSetupContextDoesNotOwnSession(t *testing.T) {
	m := connectedPeer(t, "normal") // helper's setup context is already cancelled
	result, err := m.CallTool(context.Background(), "fixture", "echo", nil)
	if err != nil || result != "first\nsecond" {
		t.Fatalf("session died with setup context: result=%v err=%v", result, err)
	}
}

func TestClientLateReplyCannotSatisfyNextRequest(t *testing.T) {
	m := connectedPeer(t, "late-response")
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	_, err := m.CallTool(ctx, "fixture", "slow", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled request error = %v", err)
	}
	// The original reply arrives before this echo reply, and must not replace it.
	result, err := m.CallTool(context.Background(), "fixture", "echo", nil)
	if err != nil || result != "first\nsecond" {
		t.Fatalf("late reply corrupted next request: result=%v err=%v", result, err)
	}
}

func TestClientEvidenceProgressAndToolErrorsStayDistinct(t *testing.T) {
	m := connectedPeer(t, "normal")
	var observed atomic.Int32
	_, err := m.CallToolWithProgress(context.Background(), "fixture", "slow", nil, func(p ProgressUpdate) {
		if p.Message == "Inspecting fixture artifact" && p.Progress == 1 {
			observed.Add(1)
		}
	})
	if err != nil || observed.Load() != 1 {
		t.Fatalf("progress=%d err=%v", observed.Load(), err)
	}
	result, err := m.CallToolWithProgress(context.Background(), "fixture", "inspect", nil, nil)
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil {
		t.Fatal(err)
	}
	evidence := envelope["structuredContent"].(map[string]any)
	if evidence["evidence_id"] != "ev_fixture" || envelope["_meta"] == nil || evidence["limitations"] == nil {
		t.Fatalf("structured evidence or provenance lost: %s", result.Content)
	}
	failed, err := m.CallToolWithProgress(context.Background(), "fixture", "tool_error", nil, nil)
	if err != nil || !failed.IsError || !strings.Contains(failed.Content, "TARGET_MISSING") || !strings.Contains(failed.Content, "remediation") {
		t.Fatalf("tool failure lost its evidence or outcome: %+v, %v", failed, err)
	}
	if _, err := m.CallTool(context.Background(), "fixture", "tool_error", nil); err == nil {
		t.Fatal("text compatibility API reported a failed tool as success")
	}
}

func TestClientConcurrentCallsKeepTheirOwnEvidence(t *testing.T) {
	m := connectedPeer(t, "normal")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value := strings.Repeat("x", i+1)
			result, err := m.CallToolWithProgress(context.Background(), "fixture", "inspect", map[string]any{"padding": value}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			var envelope struct {
				StructuredContent struct {
					Padding string `json:"padding"`
				} `json:"structuredContent"`
			}
			if err := json.Unmarshal([]byte(result.Content), &envelope); err != nil || envelope.StructuredContent.Padding != value {
				t.Errorf("response correlation failed for call %d: %s, %v", i, result.Content, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestClientExitFailsPendingCallAndUpdatesConnection(t *testing.T) {
	m := connectedPeer(t, "exit-on-call")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := m.CallTool(ctx, "fixture", "inspect", nil); err == nil {
		t.Fatal("peer exit was reported as a successful tool call")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if conn, _ := m.GetConnection("fixture"); conn.Status == "disconnected" {
			if len(m.AllToolDefs()) != 0 {
				t.Fatal("dead server is still advertising runnable tools")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("connection stayed connected after peer exited")
}
