package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	servicemcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	toolmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/mcp"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

type permissionContractExpectation string

const (
	permissionContractAllow   permissionContractExpectation = "allow"
	permissionContractDeny    permissionContractExpectation = "deny"
	permissionContractPending permissionContractExpectation = "approval-pending"
)

type permissionContractCase struct {
	toolName    string
	expectation map[config.PermissionMode]permissionContractExpectation
}

type permissionContractOperation struct {
	name         string
	input        map[string]any
	state        func() string
	successToken string
}

type permissionContractOutcome struct {
	events []types.StreamEvent
}

type permissionContractRun struct {
	globalCalls *atomic.Int32
	done        <-chan permissionContractOutcome
}

func TestProductionPermissionContractAcrossModes(t *testing.T) {
	// Keep approval behavior interactive in every permission mode so an Ask
	// decision is observable and its pre-execution side-effect boundary can be
	// asserted. The application's default yolo behavior for danger-full-access
	// is a separate execution-mode choice, not a permission-policy shortcut.
	modes := []config.PermissionMode{
		config.PermissionReadOnly,
		config.PermissionWorkspaceWrite,
		config.PermissionDangerFullAccess,
	}
	paths := []struct {
		name      string
		streaming bool
	}{
		{name: "streaming", streaming: true},
		{name: "batch", streaming: false},
	}
	cases := []permissionContractCase{
		{
			toolName: "read",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractAllow,
				config.PermissionWorkspaceWrite:   permissionContractAllow,
				config.PermissionDangerFullAccess: permissionContractAllow,
			},
		},
		{
			toolName: "write",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractDeny,
				config.PermissionWorkspaceWrite:   permissionContractPending,
				config.PermissionDangerFullAccess: permissionContractPending,
			},
		},
		{
			toolName: "edit",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractDeny,
				config.PermissionWorkspaceWrite:   permissionContractPending,
				config.PermissionDangerFullAccess: permissionContractPending,
			},
		},
		{
			toolName: "bash",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractDeny,
				config.PermissionWorkspaceWrite:   permissionContractDeny,
				config.PermissionDangerFullAccess: permissionContractPending,
			},
		},
		{
			toolName: "unknown_contract_tool",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractDeny,
				config.PermissionWorkspaceWrite:   permissionContractDeny,
				config.PermissionDangerFullAccess: permissionContractDeny,
			},
		},
		{
			toolName: "mcp_contract_touch",
			expectation: map[config.PermissionMode]permissionContractExpectation{
				config.PermissionReadOnly:         permissionContractDeny,
				config.PermissionWorkspaceWrite:   permissionContractPending,
				config.PermissionDangerFullAccess: permissionContractPending,
			},
		},
	}

	for _, path := range paths {
		path := path
		t.Run(path.name, func(t *testing.T) {
			for _, mode := range modes {
				mode := mode
				t.Run(mode.String(), func(t *testing.T) {
					for _, tc := range cases {
						tc := tc
						t.Run(tc.toolName, func(t *testing.T) {
							app, tuiApp := newProductionPermissionContractApp(t, mode)
							operation := newPermissionContractOperation(t, app, tc.toolName)
							initialState := operation.state()
							run := startProductionPermissionContractRun(t, app, tuiApp, operation, path.streaming)

							switch tc.expectation[mode] {
							case permissionContractAllow:
								outcome := waitForPermissionContractOutcome(t, run.done)
								assertGlobalPermissionCallCount(t, run.globalCalls, 1)
								assertPermissionContractResult(t, outcome.events, false, operation.successToken)
								if got := operation.state(); got != initialState {
									t.Errorf("read-only operation changed observable state: got %q, want %q", got, initialState)
								}

							case permissionContractDeny:
								outcome := waitForPermissionContractOutcome(t, run.done)
								assertGlobalPermissionCallCount(t, run.globalCalls, 1)
								assertPermissionContractResult(t, outcome.events, true, "")
								if got := operation.state(); got != initialState {
									t.Errorf("denied %s produced a side effect: got state %q, want %q", tc.toolName, got, initialState)
								}

							case permissionContractPending:
								req, completed := waitForPermissionContractApproval(t, tuiApp, run.done)
								if completed != nil {
									assertGlobalPermissionCallCount(t, run.globalCalls, 1)
									if got := operation.state(); got != initialState {
										t.Errorf("%s completed without approval and changed state: got %q, want %q", tc.toolName, got, initialState)
									}
									t.Errorf("%s completed without producing an approval request", tc.toolName)
									return
								}
								if req == nil {
									t.Errorf("%s did not produce an approval request", tc.toolName)
									return
								}

								assertGlobalPermissionCallCount(t, run.globalCalls, 1)
								if got := operation.state(); got != initialState {
									t.Errorf("approval-pending %s produced a side effect: got state %q, want %q", tc.toolName, got, initialState)
								}

								req.Respond(approval.DecisionReject)
								outcome := waitForPermissionContractOutcome(t, run.done)
								assertPermissionContractResult(t, outcome.events, true, "")
								if got := operation.state(); got != initialState {
									t.Errorf("rejected %s produced a side effect: got state %q, want %q", tc.toolName, got, initialState)
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestProductionPermissionContractAlwaysDenyHasFirstPrecedence(t *testing.T) {
	for _, streaming := range []bool{true, false} {
		streaming := streaming
		t.Run(map[bool]string{true: "streaming", false: "batch"}[streaming], func(t *testing.T) {
			app, tuiApp := newProductionPermissionContractApp(t, config.PermissionDangerFullAccess)
			app.config.AlwaysAllow = []string{"write"}
			app.config.AlwaysDeny = []string{"write"}

			operation := newPermissionContractOperation(t, app, "write")
			initialState := operation.state()
			run := startProductionPermissionContractRun(t, app, tuiApp, operation, streaming)
			req, completed := waitForPermissionContractApproval(t, tuiApp, run.done)

			if req != nil {
				req.Respond(approval.DecisionReject)
				_ = waitForPermissionContractOutcome(t, run.done)
				t.Error("always-denied tool reached the approval checkpoint")
			} else if completed != nil {
				assertPermissionContractResult(t, completed.events, true, "")
			}

			assertGlobalPermissionCallCount(t, run.globalCalls, 1)
			if got := operation.state(); got != initialState {
				t.Errorf("always-denied write produced a side effect: got state %q, want %q", got, initialState)
			}
		})
	}
}

func TestProductionPermissionContractGranularDisableTightensDangerMode(t *testing.T) {
	for _, streaming := range []bool{true, false} {
		streaming := streaming
		t.Run(map[bool]string{true: "streaming", false: "batch"}[streaming], func(t *testing.T) {
			app, tuiApp := newProductionPermissionContractApp(t, config.PermissionDangerFullAccess)
			app.config.PermWrite = false

			operation := newPermissionContractOperation(t, app, "write")
			initialState := operation.state()
			run := startProductionPermissionContractRun(t, app, tuiApp, operation, streaming)
			req, completed := waitForPermissionContractApproval(t, tuiApp, run.done)

			if req != nil {
				req.Respond(approval.DecisionReject)
				_ = waitForPermissionContractOutcome(t, run.done)
				t.Error("granularly disabled write reached the approval checkpoint")
			} else if completed != nil {
				assertPermissionContractResult(t, completed.events, true, "")
			}

			assertGlobalPermissionCallCount(t, run.globalCalls, 1)
			if got := operation.state(); got != initialState {
				t.Errorf("granularly disabled write produced a side effect: got state %q, want %q", got, initialState)
			}
		})
	}
}

func TestProductionAuditContractReadLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		operation  func(t *testing.T, app *App) permissionContractOperation
		wantError  bool
		finalEvent string
	}{
		{
			name: "success",
			operation: func(t *testing.T, app *App) permissionContractOperation {
				return newPermissionContractOperation(t, app, "read")
			},
			finalEvent: "success",
		},
		{
			name: "failure",
			operation: func(t *testing.T, app *App) permissionContractOperation {
				path := filepath.Join(app.cwd, "missing-read-source.txt")
				return permissionContractOperation{
					name:  "read",
					input: map[string]any{"file_path": path},
					state: func() string { return permissionContractFileState(path) },
				}
			},
			wantError:  true,
			finalEvent: "failure",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app, tuiApp := newProductionPermissionContractApp(t, config.PermissionReadOnly)
			operation := tc.operation(t, app)
			run := startProductionPermissionContractRun(t, app, tuiApp, operation, true)
			outcome := waitForPermissionContractOutcome(t, run.done)

			assertPermissionContractResult(t, outcome.events, tc.wantError, operation.successToken)
			entries, err := readPermissionContractAuditEntries()
			if err != nil {
				t.Fatalf("read raw audit JSONL: %v", err)
			}
			assertPermissionContractAuditLifecycle(t, entries, tc.finalEvent)
		})
	}
}

func TestProductionAuditContractDeniedAndPendingNeverStartEarly(t *testing.T) {
	t.Run("denied", func(t *testing.T) {
		app, tuiApp := newProductionPermissionContractApp(t, config.PermissionDangerFullAccess)
		app.config.PermRead = false

		operation := newPermissionContractOperation(t, app, "read")
		run := startProductionPermissionContractRun(t, app, tuiApp, operation, true)
		_ = waitForPermissionContractOutcome(t, run.done)

		entries, err := readPermissionContractAuditEntries()
		if err != nil {
			t.Fatalf("read denied audit JSONL: %v", err)
		}
		assertPermissionContractAuditDecisionWithoutStart(t, entries)
	})

	t.Run("approval-pending", func(t *testing.T) {
		app, tuiApp := newProductionPermissionContractApp(t, config.PermissionWorkspaceWrite)

		operation := newPermissionContractOperation(t, app, "write")
		run := startProductionPermissionContractRun(t, app, tuiApp, operation, true)
		req, completed := waitForPermissionContractApproval(t, tuiApp, run.done)
		if completed != nil {
			t.Error("write completed before an approval decision")
			return
		}
		if req == nil {
			t.Error("write did not produce an approval request")
			return
		}

		pendingEntries, err := readPermissionContractAuditEntries()
		if err != nil {
			t.Errorf("read approval-pending audit JSONL: %v", err)
		} else {
			assertPermissionContractAuditHasEvent(t, pendingEntries, "proposal")
			assertPermissionContractAuditHasNoEvent(t, pendingEntries, "start")
			assertPermissionContractAuditCorrelation(t, pendingEntries)
		}

		req.Respond(approval.DecisionReject)
		_ = waitForPermissionContractOutcome(t, run.done)

		rejectedEntries, err := readPermissionContractAuditEntries()
		if err != nil {
			t.Fatalf("read rejected audit JSONL: %v", err)
		}
		assertPermissionContractAuditDecisionWithoutStart(t, rejectedEntries)
	})
}

func newProductionPermissionContractApp(t *testing.T, mode config.PermissionMode) (*App, *tui.App) {
	t.Helper()

	root := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("AGENT_HARNESS_SESSION_DIR", filepath.Join(root, "sessions"))
	t.Setenv("AH_PROVIDER", "local")
	t.Setenv("AH_API_KEY", "local")
	t.Setenv("AH_MODEL", "permission-contract-model")
	t.Setenv("AH_PERMISSION_MODE", mode.String())
	t.Setenv("AH_EXECUTION_MODE", "interactive")

	switch mode {
	case config.PermissionReadOnly:
		t.Setenv("AH_PERM_READ", "true")
		t.Setenv("AH_PERM_WRITE", "false")
		t.Setenv("AH_PERM_DELETE", "false")
		t.Setenv("AH_PERM_EXECUTE", "false")
	case config.PermissionWorkspaceWrite:
		t.Setenv("AH_PERM_READ", "true")
		t.Setenv("AH_PERM_WRITE", "true")
		t.Setenv("AH_PERM_DELETE", "false")
		t.Setenv("AH_PERM_EXECUTE", "false")
	case config.PermissionDangerFullAccess:
		t.Setenv("AH_PERM_READ", "true")
		t.Setenv("AH_PERM_WRITE", "true")
		t.Setenv("AH_PERM_DELETE", "true")
		t.Setenv("AH_PERM_EXECUTE", "true")
	}

	app, err := newApp()
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp

	if app.config.PermissionMode != mode {
		t.Fatalf("newApp permission mode = %s, want %s", app.config.PermissionMode, mode)
	}
	if app.executionMode != approval.ModeInteractive {
		t.Fatalf("newApp execution mode = %s, want interactive", app.executionMode)
	}

	return app, tuiApp
}

func newPermissionContractOperation(t *testing.T, app *App, toolName string) permissionContractOperation {
	t.Helper()

	switch toolName {
	case "read":
		path := filepath.Join(app.cwd, "read-source.txt")
		if err := os.WriteFile(path, []byte("permission-contract-read"), 0o644); err != nil {
			t.Fatalf("seed read file: %v", err)
		}
		return permissionContractOperation{
			name:         toolName,
			input:        map[string]any{"file_path": path},
			state:        func() string { return permissionContractFileState(path) },
			successToken: "permission-contract-read",
		}

	case "write":
		path := filepath.Join(app.cwd, "write-target.txt")
		return permissionContractOperation{
			name:         toolName,
			input:        map[string]any{"file_path": path, "content": "permission-contract-write"},
			state:        func() string { return permissionContractFileState(path) },
			successToken: "Wrote ",
		}

	case "edit":
		path := filepath.Join(app.cwd, "edit-target.txt")
		if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
			t.Fatalf("seed edit file: %v", err)
		}
		return permissionContractOperation{
			name: toolName,
			input: map[string]any{
				"file_path":  path,
				"old_string": "before",
				"new_string": "after",
			},
			state:        func() string { return permissionContractFileState(path) },
			successToken: "Edited ",
		}

	case "bash":
		path := filepath.Join(app.cwd, "bash-marker.txt")
		command := fmt.Sprintf("printf 'permission-contract-bash' > %s", permissionContractShellQuote(path))
		return permissionContractOperation{
			name:         toolName,
			input:        map[string]any{"command": command},
			state:        func() string { return permissionContractFileState(path) },
			successToken: "",
		}

	case "unknown_contract_tool":
		path := filepath.Join(app.cwd, "unknown-marker.txt")
		return permissionContractOperation{
			name:  toolName,
			input: map[string]any{},
			state: func() string { return permissionContractFileState(path) },
		}

	case "mcp_contract_touch":
		path := filepath.Join(app.cwd, "mcp-marker.txt")
		manager := servicemcp.NewManager()
		wrapped := toolmcp.Wrap(servicemcp.WrappedToolDef{
			ServerName: "contract",
			ToolDef: servicemcp.ToolDef{
				Name:        "touch",
				Description: "records whether the MCP call boundary was reached",
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			},
		}, manager)
		call := wrapped.Call
		wrapped.Call = func(input map[string]any, ctx tools.Context, canUseTool tools.CanUseToolFn, onProgress tools.OnProgress) (tools.ToolResult, error) {
			if err := os.WriteFile(path, []byte("permission-contract-mcp"), 0o644); err != nil {
				return tools.ToolResult{}, err
			}
			return call(input, ctx, canUseTool, onProgress)
		}
		app.toolRegistry.RegisterMCP(wrapped)

		return permissionContractOperation{
			name:  toolName,
			input: map[string]any{},
			state: func() string { return permissionContractFileState(path) },
		}

	default:
		t.Fatalf("unsupported permission contract tool %q", toolName)
		return permissionContractOperation{}
	}
}

func startProductionPermissionContractRun(
	t *testing.T,
	app *App,
	tuiApp *tui.App,
	operation permissionContractOperation,
	streaming bool,
) permissionContractRun {
	t.Helper()

	inputJSON, err := json.Marshal(operation.input)
	if err != nil {
		t.Fatalf("marshal tool input: %v", err)
	}

	mock := &llm.MockClient{Events: llm.MockToolUseResponse(operation.name, string(inputJSON))}
	app.client = mock
	app.loop = agent.NewLoop(mock)
	app.loop.Config.StreamingToolExecution = streaming

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	basePermission := app.createToolPermissionFunc(tuiApp)
	var globalCalls atomic.Int32
	canUseTool := func(toolName string, input map[string]any, toolCtx tools.Context) (tools.PermissionDecision, error) {
		globalCalls.Add(1)
		return basePermission(toolName, input, toolCtx)
	}

	toolCtx := tools.Context{
		Options: tools.Options{
			MainLoopModel: app.session.Model,
			Tools:         app.toolRegistry.FilterEnabled(),
			Debug:         false,
		},
		AbortController: ctx,
	}
	stream, err := app.loop.Query(ctx, agent.QueryParams{
		Messages:       app.session.Messages,
		CanUseTool:     canUseTool,
		ToolUseContext: toolCtx,
		MaxTurns:       1,
	})
	if err != nil {
		t.Fatalf("Loop.Query() error = %v", err)
	}

	done := make(chan permissionContractOutcome, 1)
	go func() {
		var events []types.StreamEvent
		for event := range stream {
			events = append(events, event)
		}
		done <- permissionContractOutcome{events: events}
	}()

	return permissionContractRun{
		globalCalls: &globalCalls,
		done:        done,
	}
}

func waitForPermissionContractOutcome(t *testing.T, done <-chan permissionContractOutcome) permissionContractOutcome {
	t.Helper()

	select {
	case outcome := <-done:
		return outcome
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for production agent loop")
		return permissionContractOutcome{}
	}
}

func waitForPermissionContractApproval(
	t *testing.T,
	tuiApp *tui.App,
	done <-chan permissionContractOutcome,
) (*approval.ApprovalRequest, *permissionContractOutcome) {
	t.Helper()

	msgChan := permissionContractTUIMessageChannel(t, tuiApp)
	timeout := reflect.ValueOf(time.After(3 * time.Second))
	doneValue := reflect.ValueOf(done)

	for {
		chosen, value, ok := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: msgChan},
			{Dir: reflect.SelectRecv, Chan: doneValue},
			{Dir: reflect.SelectRecv, Chan: timeout},
		})
		switch chosen {
		case 0:
			if !ok {
				t.Fatal("TUI message channel closed while waiting for approval")
			}
			if msg, ok := value.Interface().(tui.ApprovalRequestMsg); ok {
				return msg.Request, nil
			}
		case 1:
			if !ok {
				t.Fatal("agent loop result channel closed while waiting for approval")
			}
			outcome := value.Interface().(permissionContractOutcome)
			return nil, &outcome
		case 2:
			t.Fatal("timed out waiting for approval request or agent-loop completion")
		}
	}
}

func permissionContractTUIMessageChannel(t *testing.T, app *tui.App) reflect.Value {
	t.Helper()

	field := reflect.ValueOf(app).Elem().FieldByName("msgChan")
	if !field.IsValid() {
		t.Fatal("tui.App.msgChan field not found")
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func assertGlobalPermissionCallCount(t *testing.T, calls *atomic.Int32, want int32) {
	t.Helper()
	if got := calls.Load(); got != want {
		t.Errorf("global permission policy call count = %d, want %d", got, want)
	}
}

func assertPermissionContractResult(t *testing.T, events []types.StreamEvent, wantError bool, token string) {
	t.Helper()

	var results []types.ToolResultBlock
	for _, event := range events {
		message, ok := event.(types.StreamMessage)
		if !ok {
			continue
		}
		for _, block := range message.Message.Content {
			if result, ok := block.(types.ToolResultBlock); ok {
				results = append(results, result)
			}
		}
	}

	if len(results) != 1 {
		t.Errorf("tool result count = %d, want 1", len(results))
		return
	}
	if results[0].IsError != wantError {
		t.Errorf("tool result IsError = %v, want %v (content: %v)", results[0].IsError, wantError, results[0].Content)
	}
	if token != "" && !strings.Contains(fmt.Sprintf("%v", results[0].Content), token) {
		t.Errorf("tool result %q does not contain %q", results[0].Content, token)
	}
}

func readPermissionContractAuditEntries() ([]map[string]any, error) {
	paths, err := filepath.Glob(filepath.Join(config.DataAudit(), "*.log"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no audit log found under %s", config.DataAudit())
	}
	sort.Strings(paths)

	var entries []map[string]any
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			if strings.TrimSpace(scanner.Text()) == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("%s:%d: %w", path, line, err)
			}
			entries = append(entries, entry)
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("audit log under %s contained no entries", config.DataAudit())
	}
	return entries, nil
}

func assertPermissionContractAuditLifecycle(t *testing.T, entries []map[string]any, finalEvent string) {
	t.Helper()

	wantEvents := []string{"proposal", "decision", "start", finalEvent}
	if len(entries) != len(wantEvents) {
		t.Errorf("audit event count = %d, want %d", len(entries), len(wantEvents))
	}

	for i, want := range wantEvents {
		if i >= len(entries) {
			break
		}
		if got := permissionContractAuditEvent(entries[i]); got != want {
			t.Errorf("audit event[%d] = %q, want %q", i, got, want)
		}
	}
	assertPermissionContractAuditCorrelation(t, entries)

	if len(entries) == 0 {
		return
	}
	outcome := entries[len(entries)-1]
	duration, ok := outcome["duration_ms"]
	if !ok {
		t.Errorf("%s audit event has no duration_ms field", finalEvent)
		return
	}
	number, ok := duration.(float64)
	if !ok {
		t.Errorf("%s audit duration_ms type = %T, want JSON number", finalEvent, duration)
		return
	}
	if number < 0 {
		t.Errorf("%s audit duration_ms = %v, want non-negative", finalEvent, number)
	}
}

func assertPermissionContractAuditDecisionWithoutStart(t *testing.T, entries []map[string]any) {
	t.Helper()

	proposalIndex := permissionContractAuditEventIndex(entries, "proposal")
	decisionIndex := permissionContractAuditEventIndex(entries, "decision")
	if proposalIndex < 0 {
		t.Error("audit log has no proposal event")
	}
	if decisionIndex < 0 {
		t.Error("audit log has no decision event")
	}
	if proposalIndex >= 0 && decisionIndex >= 0 && proposalIndex >= decisionIndex {
		t.Errorf("audit proposal index = %d, decision index = %d; want proposal before decision", proposalIndex, decisionIndex)
	}
	assertPermissionContractAuditHasNoEvent(t, entries, "start")
	assertPermissionContractAuditCorrelation(t, entries)
}

func assertPermissionContractAuditHasEvent(t *testing.T, entries []map[string]any, event string) {
	t.Helper()
	if permissionContractAuditEventIndex(entries, event) < 0 {
		t.Errorf("audit log has no %s event", event)
	}
}

func assertPermissionContractAuditHasNoEvent(t *testing.T, entries []map[string]any, event string) {
	t.Helper()
	if index := permissionContractAuditEventIndex(entries, event); index >= 0 {
		t.Errorf("audit log contains premature %s event at index %d", event, index)
	}
}

func assertPermissionContractAuditCorrelation(t *testing.T, entries []map[string]any) {
	t.Helper()
	if len(entries) == 0 {
		t.Error("audit log has no correlated entries")
		return
	}

	var correlationID string
	for i, entry := range entries {
		id, ok := entry["tool_call_id"].(string)
		if !ok || id == "" {
			t.Errorf("audit event[%d] tool_call_id = %v, want nonempty string", i, entry["tool_call_id"])
			continue
		}
		if correlationID == "" {
			correlationID = id
			continue
		}
		if id != correlationID {
			t.Errorf("audit event[%d] tool_call_id = %q, want %q", i, id, correlationID)
		}
	}
}

func permissionContractAuditEventIndex(entries []map[string]any, event string) int {
	for i, entry := range entries {
		if permissionContractAuditEvent(entry) == event {
			return i
		}
	}
	return -1
}

func permissionContractAuditEvent(entry map[string]any) string {
	event, _ := entry["event"].(string)
	return event
}

func permissionContractFileState(path string) string {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "<absent>"
	}
	if err != nil {
		return "<error: " + err.Error() + ">"
	}
	return string(data)
}

func permissionContractShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
