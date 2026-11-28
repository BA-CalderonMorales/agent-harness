package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
)

// readOnlyPermissionApp returns an app in read-only mode with only the read
// toggle on, the shape a fresh read-only session starts with.
func readOnlyPermissionApp(t *testing.T) *App {
	t.Helper()
	return &App{config: &config.LayeredConfig{
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
	}}
}

// TestReadOnlyModeAllowsTodoWrite pins that the plan tool stays usable in
// read-only mode: it only updates the in-session checklist and writes no
// files, so denying it blocked the planning step the prompt asks for.
func TestReadOnlyModeAllowsTodoWrite(t *testing.T) {
	app := readOnlyPermissionApp(t)
	decision := app.checkPermissionMode("todo_write")
	if decision.Behavior != tools.Allow {
		t.Fatalf("todo_write in read-only mode = %s (%s), want allow", decision.Behavior, decision.Message)
	}
}

// TestReadOnlyModeAllowsTodoWriteThroughPermissionFunc exercises the full
// permission path the agent loop uses, not just the mode switch, and checks
// the plan tool still runs and echoes the plan it wrote.
func TestReadOnlyModeAllowsTodoWriteThroughPermissionFunc(t *testing.T) {
	app := readOnlyPermissionApp(t)
	app.agentMode = AgentModeAuto
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(builtin.TodoWriteTool)

	canUse := app.createToolPermissionFunc(tui.NewApp())
	input := map[string]any{"todos": []any{
		map[string]any{"text": "read the permission code", "status": "in_progress"},
	}}
	decision, err := canUse("todo_write", input, tools.Context{})
	if err != nil {
		t.Fatalf("permission check error: %v", err)
	}
	if decision.Behavior != tools.Allow {
		t.Fatalf("todo_write through permission func = %s (%s), want allow", decision.Behavior, decision.Message)
	}

	result, err := builtin.TodoWriteTool.Call(input, tools.Context{}, canUse, nil)
	if err != nil {
		t.Fatalf("todo_write call error: %v", err)
	}
	if got, _ := result.Data.(string); !strings.HasPrefix(got, "Updated 1 todos") {
		t.Fatalf("todo_write did not echo the plan: %v", result.Data)
	}
}

// TestReadOnlyModeKeepsWritesAndShellDenied pins that, with every write-class
// toggle off, file writes, shell, delete and MCP tools are denied in read-only
// mode, while todo_write is allowed. The granular toggles run first, so the
// mode switch is only reached for tools with no toggle (MCP, todo_write).
func TestReadOnlyModeKeepsWritesAndShellDenied(t *testing.T) {
	for _, name := range []string{"write", "edit", "bash", "shell", "execute_command", "delete", "mcp_contract_touch"} {
		t.Run(name, func(t *testing.T) {
			app := readOnlyPermissionApp(t)
			if decision := app.checkPermissionMode(name); decision.Behavior != tools.Deny {
				t.Fatalf("%s in read-only mode = %s, want deny", name, decision.Behavior)
			}
		})
	}
	t.Run("todo_write_allowed", func(t *testing.T) {
		app := readOnlyPermissionApp(t)
		if decision := app.checkPermissionMode("todo_write"); decision.Behavior != tools.Allow {
			t.Fatalf("todo_write in read-only mode = %s, want allow", decision.Behavior)
		}
	})
}

// TestReadOnlyModeWriteToggleOverridesPreset pins the documented rule that an
// explicitly enabled toggle outranks the read-only preset, and that the todo
// allowance does not change it: with Write and Execute on, those tools pass
// the mode switch, and with the toggles off they are denied at the toggle.
func TestReadOnlyModeWriteToggleOverridesPreset(t *testing.T) {
	on := &App{config: &config.LayeredConfig{
		PermissionMode: config.PermissionReadOnly,
		PermRead:       true,
		PermWrite:      true,
		PermExecute:    true,
	}}
	for _, name := range []string{"write", "edit", "bash"} {
		if decision := on.checkPermissionMode(name); decision.Behavior != tools.Allow {
			t.Errorf("%s with its toggle on = %s, want allow", name, decision.Behavior)
		}
	}
	off := readOnlyPermissionApp(t)
	for _, name := range []string{"write", "edit", "bash"} {
		if decision := off.checkPermissionMode(name); decision.Behavior != tools.Deny {
			t.Errorf("%s with its toggle off = %s, want deny", name, decision.Behavior)
		}
	}
}

// TestStateOnlyToolListIsExactlyTodoWrite pins the full contents of the
// allowance list, not a sample of names. Write and shell tools are covered by
// granular toggles, so a mutation that adds them is behaviorally invisible
// through checkPermissionMode; comparing the whole list is what catches it.
func TestStateOnlyToolListIsExactlyTodoWrite(t *testing.T) {
	want := []string{"todo_write"}
	if !reflect.DeepEqual(stateOnlyTools, want) {
		t.Fatalf("stateOnlyTools = %v, want exactly %v", stateOnlyTools, want)
	}
}

// TestPlanModeStillDeniesTodoWrite pins that plan mode is unchanged: its gate
// reads isReadOnlyTool, not the read-only allowance, so todo_write stays denied
// there.
func TestPlanModeStillDeniesTodoWrite(t *testing.T) {
	app := readOnlyPermissionApp(t)
	app.agentMode = AgentModePlan
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(builtin.TodoWriteTool)

	canUse := app.createToolPermissionFunc(tui.NewApp())
	decision, err := canUse("todo_write", map[string]any{"todos": []any{}}, tools.Context{})
	if err != nil {
		t.Fatalf("permission check error: %v", err)
	}
	if decision.Behavior != tools.Deny {
		t.Fatalf("todo_write in plan mode = %s, want deny", decision.Behavior)
	}
}

// TestReadOnlyModeDeniesUnrelatedNonReadOnlyTool pins that the allowance is
// named, not a general relaxation: an unknown write-class tool is still denied
// through the full permission path.
func TestReadOnlyModeDeniesUnrelatedNonReadOnlyTool(t *testing.T) {
	app := readOnlyPermissionApp(t)
	app.agentMode = AgentModeAuto
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(tools.Tool{Name: "unknown_write_tool"})

	canUse := app.createToolPermissionFunc(tui.NewApp())
	decision, err := canUse("unknown_write_tool", map[string]any{}, tools.Context{})
	if err != nil {
		t.Fatalf("permission check error: %v", err)
	}
	if decision.Behavior != tools.Deny {
		t.Fatalf("unrelated write tool in read-only mode = %s, want deny", decision.Behavior)
	}
}

func TestGranularReadToggleCoversDirectoryListingAndFind(t *testing.T) {
	app := &App{config: &config.LayeredConfig{
		PermissionMode: config.PermissionWorkspaceWrite,
		PermRead:       false,
		PermWrite:      true,
		PermExplicit:   true,
	}}
	for _, name := range []string{"ls", "ls_recursive", "find", "read", "glob", "grep"} {
		if decision := app.checkPermissionMode(name); decision.Behavior != tools.Deny {
			t.Errorf("%s with PermRead=false = %s, want deny", name, decision.Behavior)
		}
	}
}

func TestSyncGranularPermissionsRespectsExplicitAllOff(t *testing.T) {
	app := &App{config: &config.LayeredConfig{
		PermissionMode: config.PermissionDangerFullAccess,
		PermExplicit:   true,
	}}
	app.syncGranularPermissions()
	if app.config.PermRead || app.config.PermWrite || app.config.PermDelete || app.config.PermExecute {
		t.Fatalf("syncGranularPermissions overrode explicit all-off toggles: %+v", app.config)
	}
}
