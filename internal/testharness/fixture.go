package testharness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/permissions"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Fixture builds a headless agent-harness app for behavior testing.
// It initializes real production components without TUI.
type Fixture struct {
	T              *testing.T
	WorkDir        string
	Config         *config.LayeredConfig
	Session        *state.Session
	SessionManager *state.SessionManager
	ToolRegistry   *tools.ToolRegistry
	MockLLM        *llm.MockClient
	Loop           *agent.Loop

	permCtx      permissions.Context
	lastDecision tools.PermissionDecision
	lastEvents   []types.StreamEvent
}

// NewFixture creates a fresh test fixture with all core components initialized.
func NewFixture(t *testing.T) *Fixture {
	t.Helper()

	workDir := t.TempDir()

	// Minimal config with temp directories
	cfg := &config.LayeredConfig{
		Provider:    "test",
		Model:       "test-model",
		AlwaysAllow: []string{},
		AlwaysDeny:  []string{},
	}

	// Session manager with temp session dir
	sessionDir := filepath.Join(workDir, "sessions")
	os.MkdirAll(sessionDir, 0755)
	os.Setenv("HOME", workDir) // redirect session storage to temp dir

	sm, err := state.NewSessionManager()
	if err != nil {
		t.Fatalf("init session manager: %v", err)
	}

	// Tool registry with all builtins
	reg := tools.NewRegistry()
	reg.RegisterBuiltIn(builtin.BashTool)
	reg.RegisterBuiltIn(builtin.FileReadTool)
	reg.RegisterBuiltIn(builtin.FileWriteTool)
	reg.RegisterBuiltIn(builtin.GlobTool)
	reg.RegisterBuiltIn(builtin.GrepTool)
	reg.RegisterBuiltIn(builtin.LsRecursiveTool)
	reg.RegisterBuiltIn(builtin.ListDirectoryTool)
	reg.RegisterBuiltIn(builtin.FindTool)
	reg.RegisterBuiltIn(builtin.FileEditTool)
	reg.RegisterBuiltIn(builtin.AskUserQuestionTool)
	reg.RegisterBuiltIn(builtin.TodoWriteTool)
	reg.RegisterBuiltIn(builtin.WebFetchTool)
	reg.RegisterBuiltIn(builtin.WebSearchTool)
	reg.RegisterBuiltIn(builtin.AgentTool)
	reg.RegisterBuiltIn(builtin.ExportTool)
	reg.RegisterBuiltIn(builtin.NotebookEditTool)
	reg.RegisterBuiltIn(builtin.EnterPlanModeTool)
	reg.RegisterBuiltIn(builtin.ExitPlanModeTool)
	reg.RegisterBuiltIn(builtin.RewindTool)
	reg.RegisterBuiltIn(builtin.SearchTranscriptTool)
	reg.RegisterBuiltIn(builtin.SettingsTool)

	mockLLM := &llm.MockClient{}
	loop := agent.NewLoop(mockLLM)
	loop.Config.DefaultMaxTurns = 5

	f := &Fixture{
		T:              t,
		WorkDir:        workDir,
		Config:         cfg,
		SessionManager: sm,
		ToolRegistry:   reg,
		MockLLM:        mockLLM,
		Loop:           loop,
		permCtx:        permissions.EmptyContext(),
	}

	f.SyncPermissions()
	return f
}

// SyncPermissions rebuilds the permission context from fixture config.
func (f *Fixture) SyncPermissions() {
	f.permCtx = permissions.Context{
		Mode: permissions.ModeDefault,
		AlwaysAllowRules: map[permissions.RuleSource][]permissions.PermissionRule{
			permissions.SourceUserSettings: f.rulesFromList(f.Config.AlwaysAllow, tools.Allow),
		},
		AlwaysDenyRules: map[permissions.RuleSource][]permissions.PermissionRule{
			permissions.SourceUserSettings: f.rulesFromList(f.Config.AlwaysDeny, tools.Deny),
		},
		AlwaysAskRules: map[permissions.RuleSource][]permissions.PermissionRule{},
	}
}

func (f *Fixture) rulesFromList(names []string, behavior tools.DecisionBehavior) []permissions.PermissionRule {
	var rules []permissions.PermissionRule
	for _, name := range names {
		rules = append(rules, permissions.PermissionRule{
			ToolName: name,
			Behavior: behavior,
		})
	}
	return rules
}

// SetPermissionMode updates the fixture's permission mode.
func (f *Fixture) SetPermissionMode(mode permissions.Mode) {
	f.SyncPermissions()
}

// SetAlwaysAsk adds a tool to the always-ask rules.
func (f *Fixture) SetAlwaysAsk(toolName string) {
	f.permCtx.AlwaysAskRules[permissions.SourceUserSettings] = append(
		f.permCtx.AlwaysAskRules[permissions.SourceUserSettings],
		permissions.PermissionRule{ToolName: toolName, Behavior: tools.Ask},
	)
}

// SetAlwaysAllow adds a tool to the always-allow rules.
func (f *Fixture) SetAlwaysAllow(toolName string) {
	f.permCtx.AlwaysAllowRules[permissions.SourceUserSettings] = append(
		f.permCtx.AlwaysAllowRules[permissions.SourceUserSettings],
		permissions.PermissionRule{ToolName: toolName, Behavior: tools.Allow},
	)
}

// SetAlwaysDeny adds a tool to the always-deny rules.
func (f *Fixture) SetAlwaysDeny(toolName string) {
	f.permCtx.AlwaysDenyRules[permissions.SourceUserSettings] = append(
		f.permCtx.AlwaysDenyRules[permissions.SourceUserSettings],
		permissions.PermissionRule{ToolName: toolName, Behavior: tools.Deny},
	)
}

// ExecuteTool finds a tool by name, checks permissions, and executes it.
func (f *Fixture) ExecuteTool(name string, input map[string]any) error {
	tool, ok := f.ToolRegistry.FindToolByName(name)
	if !ok {
		return fmt.Errorf("tool not found: %s", name)
	}

	// Validate input
	if tool.ValidateInput != nil {
		vr := tool.ValidateInput(input, f.toolCtx())
		if !vr.Valid {
			return fmt.Errorf("validation failed: %s", vr.Message)
		}
	}

	// Check permissions via real engine
	decision := permissions.Evaluate(tool, input, f.permCtx)
	f.lastDecision = decision
	if decision.Behavior == tools.Deny {
		return fmt.Errorf("permission denied")
	}
	if decision.Behavior == tools.Ask {
		return fmt.Errorf("permission ask")
	}

	// Execute
	ctx := f.toolCtx()
	if decision.UpdatedInput != nil {
		input = decision.UpdatedInput
	}
	_, err := tool.Call(input, ctx, nil, nil)
	return err
}

// QueryLoop runs the agent loop with the configured mock LLM.
func (f *Fixture) QueryLoop(messages []types.Message, systemPrompt string) []types.StreamEvent {
	params := agent.QueryParams{
		Messages:     messages,
		SystemPrompt: systemPrompt,
		CanUseTool:   f.canUseToolFn(),
		ToolUseContext: tools.Context{
			Options: tools.Options{
				MainLoopModel: "test-model",
				Tools:         f.ToolRegistry.AllTools(),
			},
			AbortController: context.Background(),
			GlobLimits:      tools.GlobLimits{MaxResults: 100},
		},
	}

	stream, err := f.Loop.Query(context.Background(), params)
	if err != nil {
		f.T.Fatalf("loop query failed: %v", err)
	}

	var events []types.StreamEvent
	for ev := range stream {
		events = append(events, ev)
	}
	f.lastEvents = events
	return events
}

// LastDecision returns the most recent permission decision.
func (f *Fixture) LastDecision() tools.PermissionDecision {
	return f.lastDecision
}

// LastEvents returns the most recent stream events from QueryLoop.
func (f *Fixture) LastEvents() []types.StreamEvent {
	return f.lastEvents
}

// WorkDirFile returns an absolute path inside the fixture's temp work dir.
func (f *Fixture) WorkDirFile(name string) string {
	return filepath.Join(f.WorkDir, name)
}

// SessionFileExists checks if the current session has a persisted file.
func (f *Fixture) SessionFileExists() bool {
	if f.Session == nil {
		return false
	}
	path := filepath.Join(f.WorkDir, "sessions", f.Session.ID+".json")
	_, err := os.Stat(path)
	return err == nil
}

// toolCtx builds a tools.Context for tool execution.
func (f *Fixture) toolCtx() tools.Context {
	return tools.Context{
		AbortController: context.Background(),
		GlobLimits:      tools.GlobLimits{MaxResults: 100},
		Options: tools.Options{
			MainLoopModel: f.Config.Model,
			Tools:         f.ToolRegistry.AllTools(),
		},
	}
}

// canUseToolFn returns the permission checker used by the agent loop.
func (f *Fixture) canUseToolFn() tools.CanUseToolFn {
	return func(toolName string, input map[string]any, ctx tools.Context) (tools.PermissionDecision, error) {
		tool, ok := f.ToolRegistry.FindToolByName(toolName)
		if !ok {
			return tools.PermissionDecision{Behavior: tools.Deny}, nil
		}
		decision := permissions.Evaluate(tool, input, f.permCtx)
		f.lastDecision = decision
		return decision, nil
	}
}
