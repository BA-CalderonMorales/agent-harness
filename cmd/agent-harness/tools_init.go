package main

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/services/mcp"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/builtin"
	toolmcp "github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools/mcp"
	"os"
	"time"
)

// initTools registers all built-in tools and MCP tools.
func (app *App) initTools() {
	app.toolRegistry = tools.NewRegistry()
	app.toolRegistry.RegisterBuiltIn(builtin.BashTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileReadTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileEditTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FileWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GlobTool)
	app.toolRegistry.RegisterBuiltIn(builtin.GrepTool)
	app.toolRegistry.RegisterBuiltIn(builtin.LsRecursiveTool)
	app.toolRegistry.RegisterBuiltIn(builtin.ListDirectoryTool)
	app.toolRegistry.RegisterBuiltIn(builtin.FindTool)
	app.toolRegistry.RegisterBuiltIn(builtin.AskUserQuestionTool)
	app.toolRegistry.RegisterBuiltIn(builtin.TodoWriteTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebFetchTool)
	app.toolRegistry.RegisterBuiltIn(builtin.WebSearchTool)

	app.mcpManager = mcp.NewManager()
	if len(app.config.McpServers) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := app.mcpManager.LoadAndConnect(ctx, app.config.McpServers); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to connect MCP servers: %v\n", err)
		} else {
			for _, def := range app.mcpManager.AllToolDefs() {
				app.toolRegistry.RegisterMCP(toolmcp.Wrap(def, app.mcpManager))
			}
		}
	}
}

// initCommands registers all slash commands.
