package main

import (
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
)

func TestProductionRegistryIncludesAgentTool(t *testing.T) {
	app := &App{config: &config.LayeredConfig{}}
	app.initTools()
	if _, ok := app.toolRegistry.FindToolByName(builtin.AgentTool.Name); !ok {
		t.Fatalf("production registry does not contain %q", builtin.AgentTool.Name)
	}
}
