package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
)

// agentMode is the single knob users cycle with Shift+Tab in the composer.
// It unifies execution mode (approval prompting), plan gating, and tool
// availability into four honest presets:
//
//	manual — every tool call asks (safest default)
//	auto   — tool calls approve without asking, inside permission gates
//	plan   — read-only tools; the agent plans before acting
//	chat   — no tools at all; small requests, fast turns
type agentMode string

const (
	AgentModeManual agentMode = "manual"
	AgentModeAuto   agentMode = "auto"
	AgentModePlan   agentMode = "plan"
	AgentModeChat   agentMode = "chat"
)

// applyAgentMode maps an agent mode onto the machinery it controls:
// execution mode (approval dialogs), plan-mode prompt guidance, and the
// chat-mode no-tools switch. It is the single authoritative writer for
// these three fields — never set them independently elsewhere.
func (app *App) applyAgentMode(m agentMode) {
	app.agentMode = m
	switch m {
	case AgentModeAuto:
		app.executionMode = approval.ModeYolo
		app.planMode = false
	case AgentModePlan:
		app.executionMode = approval.ModeInteractive
		app.planMode = true
	default: // manual and chat both ask before tools run
		app.executionMode = approval.ModeInteractive
		app.planMode = false
	}
}

// syncAgentMode derives the agent mode from boot-time config so the first
// Shift+Tab press starts from where the settings put us.
func (app *App) syncAgentMode() {
	if app.executionMode == approval.ModeYolo {
		app.agentMode = AgentModeAuto
	} else {
		app.agentMode = AgentModeManual
	}
}

// enabledToolsForMode returns the tool set for the current agent mode.
// Chat mode sends no tools at all: the request sheds the tool schema
// (tens of kilobytes), which is the difference between a snappy local
// turn and a minute of prompt-eval dead air.
func (app *App) enabledToolsForMode() []tools.Tool {
	if app.agentMode == AgentModeChat {
		return nil
	}
	return app.toolRegistry.FilterEnabled()
}

// agentModeDescription returns the one-line flash shown when cycling.
func agentModeDescription(m agentMode) string {
	switch m {
	case AgentModeAuto:
		return "auto — tool calls approved without asking"
	case AgentModePlan:
		return "plan — read-only; the agent plans before acting"
	case AgentModeChat:
		return "chat — conversation only, no tools sent"
	default:
		return "manual — every tool call asks for approval"
	}
}
