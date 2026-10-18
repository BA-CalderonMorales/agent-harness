package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	svcmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	mcptool "github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/mcppeer"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestMCPPeerProcess(t *testing.T) { mcppeer.RunIfRequested() }

type evidenceClient struct {
	requests   []llm.Request
	name, args string
}

func (c *evidenceClient) Stream(ctx context.Context, req llm.Request) (<-chan types.LLMEvent, error) {
	c.requests = append(c.requests, req)
	events := llm.MockTextResponse("Inspection finished.")
	if len(c.requests) == 1 {
		events = llm.MockToolUseResponse(c.name, c.args)
	}
	return (&llm.MockClient{Events: events}).Stream(ctx, req)
}

func TestMCPEvidenceSurvivesApprovalBudgetModelAndResume(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			for _, bounded := range []bool{false, true} {
				t.Run(fmt.Sprintf("stream=%t/error=%t/receipt=%t", streaming, failed, bounded), func(t *testing.T) {
					testenv.SetHome(t, t.TempDir())
					t.Cleanup(tools.ResetBudgetForNewTurn)
					mgr := svcmcp.NewManager()
					t.Cleanup(func() { _ = mgr.Close() })
					command, args, err := mcppeer.Command()
					if err != nil {
						t.Fatal(err)
					}
					if err := mgr.Connect(context.Background(), "fixture", svcmcp.McpServerConfig{
						Command: command, Args: args, Env: map[string]string{"AH_MCP_TEST_PEER": "normal"},
					}); err != nil {
						t.Fatal(err)
					}
					name := "inspect"
					if failed {
						name = "tool_error"
					}
					var tool tools.Tool
					for _, def := range mgr.AllToolDefs() {
						if def.Name == name {
							tool = mcptool.Wrap(def, mgr)
						}
					}
					input := map[string]any{}
					if bounded {
						tool.MaxResultSizeChars = 1024
						input["padding"] = strings.Repeat("raw evidence ", 700)
					}
					encoded, _ := json.Marshal(input)
					client := &evidenceClient{name: tool.Name, args: string(encoded)}
					loop := NewLoop(client)
					loop.Config.StreamingToolExecution = streaming
					var audits []tools.ToolAuditEvent
					checkpoints := 0
					policy := func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
						return tools.PermissionDecision{Behavior: tools.Allow,
							Checkpoint: func() (tools.PermissionDecision, error) {
								checkpoints++
								return tools.PermissionDecision{Behavior: tools.Allow}, nil
							},
							Audit: func(event tools.ToolAuditEvent) error { audits = append(audits, event); return nil },
						}, nil
					}
					stream, err := loop.Query(context.Background(), QueryParams{
						Messages:       []types.Message{{Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: "Inspect target"}}}},
						CanUseTool:     policy,
						ToolUseContext: tools.Context{AbortController: context.Background(), Options: tools.Options{Tools: []tools.Tool{tool}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					var result types.ToolResultBlock
					for event := range stream {
						if e, ok := event.(types.StreamError); ok {
							t.Fatal(e.Error)
						}
						if e, ok := event.(types.StreamMessage); ok {
							for _, block := range e.Message.Content {
								if r, ok := block.(types.ToolResultBlock); ok {
									result = r
								}
							}
						}
					}
					if checkpoints != 1 {
						t.Fatalf("untrusted readOnlyHint bypassed approval: checkpoints=%d", checkpoints)
					}
					if result.ToolUseID != "tu_1" || result.IsError != failed {
						t.Fatalf("lost identity/outcome: %+v", result)
					}
					wantOutcome := "success"
					if failed {
						wantOutcome = "failure"
					}
					if len(audits) != 4 || audits[3].Event != wantOutcome {
						t.Fatalf("wrong durable outcome: %+v", audits)
					}
					full := result.Content
					if bounded {
						if len(full) > 1024 {
							t.Fatal("receipt exceeded model budget")
						}
						match := regexp.MustCompile(`\[Full result stored at ([^\]]+)\]`).FindStringSubmatch(full)
						if len(match) != 2 {
							t.Fatal("missing retrieval receipt")
						}
						data, err := os.ReadFile(match[1])
						if err != nil {
							t.Fatal(err)
						}
						sum := sha256.Sum256(data)
						if filepath.Base(match[1]) != fmt.Sprintf("%x.txt", sum) {
							t.Fatal("retained evidence hash mismatch")
						}
						full = string(data)
					}
					var envelope struct {
						Structured map[string]any `json:"structuredContent"`
						Meta       map[string]any `json:"_meta"`
						IsError    bool           `json:"isError"`
					}
					if err := json.Unmarshal([]byte(full), &envelope); err != nil {
						t.Fatal(err)
					}
					if envelope.Structured == nil || envelope.Meta["provenance"] == nil || envelope.IsError != failed {
						t.Fatal("evidence envelope lost data/provenance/outcome")
					}
					if failed && !strings.Contains(full, "supply a readable target") {
						t.Fatal("error lost recovery instruction")
					}
					if !failed && (!strings.Contains(full, "ev_fixture") || !strings.Contains(full, "static inspection only")) {
						t.Fatal("evidence identity or limitations lost")
					}
					if len(client.requests) != 2 {
						t.Fatal("model did not receive tool outcome")
					}
					var forwarded types.ToolResultBlock
					for _, msg := range client.requests[1].Messages {
						for _, block := range msg.Content {
							if r, ok := block.(types.ToolResultBlock); ok {
								forwarded = r
							}
						}
					}
					if forwarded != result {
						t.Fatal("model saw different evidence/outcome")
					}
					store, err := state.NewSessionManagerWithDir(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					session := store.CreateSession("fixture-model")
					for _, msg := range client.requests[1].Messages {
						session.AddMessage(msg)
					}
					if _, err := store.SaveCurrent(); err != nil {
						t.Fatal(err)
					}
					loaded, err := store.LoadSession(session.ID)
					if err != nil {
						t.Fatal(err)
					}
					var resumed types.ToolResultBlock
					for _, msg := range loaded.Messages {
						for _, block := range msg.Content {
							if r, ok := block.(types.ToolResultBlock); ok {
								resumed = r
							}
						}
					}
					if resumed != result {
						t.Fatal("resume changed evidence/outcome/retrieval receipt")
					}
				})
			}
		}
	}
}
