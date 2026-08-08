package config

import (
	"fmt"
)

// GetConfigReport returns a formatted report of the current configuration
func (lc *LayeredConfig) GetConfigReport() string {
	var result string

	result += "Configuration\n"
	result += fmt.Sprintf("  Provider         %s\n", lc.Provider)
	result += fmt.Sprintf("  Runtime          %s\n", lc.Runtime)
	result += fmt.Sprintf("  Model            %s\n", lc.Model)
	result += fmt.Sprintf("  Endpoint         %s\n", lc.EndpointURL)
	result += fmt.Sprintf("  Context length   %d\n", lc.ContextLength)
	result += fmt.Sprintf("  Max tokens       %d\n", lc.MaxTokens)
	result += fmt.Sprintf("  Temperature      %.2f\n", lc.Temperature)
	result += fmt.Sprintf("  Permission mode  %s\n", lc.PermissionMode.String())
	result += fmt.Sprintf("  Persona          %s\n", lc.Persona)
	result += "\n"

	result += "Loaded from\n"
	for _, entry := range lc.loadedEntries {
		result += fmt.Sprintf("  %s\n", entry.Path)
	}

	if len(lc.McpServers) > 0 {
		result += "\nMCP Servers\n"
		for name := range lc.McpServers {
			result += fmt.Sprintf("  %s\n", name)
		}
	}

	return result
}

// GetPermissionReport returns a formatted permission mode report
func (lc *LayeredConfig) GetPermissionReport() string {
	modes := []struct {
		name        string
		description string
		current     bool
	}{
		{"read-only", "Read/search tools only", lc.PermissionMode == PermissionReadOnly},
		{"workspace-write", "Edit files inside the workspace", lc.PermissionMode == PermissionWorkspaceWrite},
		{"danger-full-access", "Unrestricted tool access", lc.PermissionMode == PermissionDangerFullAccess},
	}

	result := "Permissions\n"
	result += fmt.Sprintf("  Active mode      %s\n", lc.PermissionMode.String())
	result += fmt.Sprintf("  Effect           %s\n", lc.PermissionMode.Description())
	result += "\n"
	result += "Modes\n"

	for _, mode := range modes {
		marker := "○ available"
		if mode.current {
			marker = "● current"
		}
		result += fmt.Sprintf("  %-18s %-11s %s\n", mode.name, marker, mode.description)
	}

	result += "\n"
	result += "Next\n"
	result += "  /permissions              Show the current mode\n"
	result += "  /permissions <mode>       Switch modes for subsequent tool calls\n"

	return result
}
