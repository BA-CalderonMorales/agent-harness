package mcp

import (
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Retain the full protocol result: structured evidence, resource links,
// embedded resources, and provenance all survive the harness boundary.
type InitializeResult = sdk.InitializeResult
type CallToolResult = sdk.CallToolResult

// ProgressUpdate is observational: it must never replace tool output.
type ProgressUpdate struct {
	Message  string  `json:"message,omitempty"`
	Progress float64 `json:"progress"`
	Total    float64 `json:"total,omitempty"`
}

func (p ProgressUpdate) String() string {
	if p.Message != "" {
		return p.Message
	}
	if p.Total > 0 {
		return fmt.Sprintf("Tool progress: %g / %g", p.Progress, p.Total)
	}
	return fmt.Sprintf("Tool progress: %g", p.Progress)
}
