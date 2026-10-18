package main

import (
	"path/filepath"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/mcppeer"
)

func TestMCPPeerProcess(t *testing.T) { mcppeer.RunIfRequested() }

func TestAppRegistersHealthyMCPToolsWhenAnotherServerFails(t *testing.T) {
	command, args, err := mcppeer.Command()
	if err != nil {
		t.Fatal(err)
	}
	app := &App{config: &config.LayeredConfig{McpServers: map[string]mcp.McpServerConfig{
		"broken":  {Command: filepath.Join(t.TempDir(), "missing")},
		"healthy": {Command: command, Args: args, Env: map[string]string{"AH_MCP_TEST_PEER": "normal"}},
	}}}
	app.initTools()
	t.Cleanup(func() { _ = app.mcpManager.Close() })
	if _, ok := app.toolRegistry.FindToolByName("mcp_healthy_inspect"); !ok {
		t.Fatal("startup warning hid a healthy server's tools")
	}
}
