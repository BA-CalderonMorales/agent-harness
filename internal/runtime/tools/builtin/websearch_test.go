package builtin

import (
	"context"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"strings"
	"testing"
)

func TestUnavailableSearchCannotReportPlaceholderSuccess(t *testing.T) {
	if WebSearchTool.Capabilities.IsEnabled() {
		t.Fatal("unimplemented search must not be advertised")
	}
	result, err := WebSearchTool.Call(map[string]any{"query": "Omaha weather today"}, tools.Context{AbortController: context.Background()}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "No search was performed") || result.Data != nil {
		t.Fatalf("unimplemented search claimed a result: %+v %v", result, err)
	}
}
