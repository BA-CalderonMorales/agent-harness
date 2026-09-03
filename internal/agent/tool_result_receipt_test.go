package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestPersistToolResultReceiptIsBoundedAndPrivate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	fullResult := "HEAD-" + strings.Repeat("private-output-", 2_000) + "-TAIL"
	const receiptLimit = 1_024
	receipt, err := persistToolResultReceipt(fullResult, receiptLimit)
	if err != nil {
		t.Fatalf("persistToolResultReceipt() error = %v", err)
	}
	if len(receipt) > receiptLimit {
		t.Fatalf("receipt length = %d, want at most %d", len(receipt), receiptLimit)
	}
	if !strings.Contains(receipt, "HEAD-") || !strings.Contains(receipt, "-TAIL") {
		t.Fatalf("receipt does not preserve both ends of the result:\n%s", receipt)
	}

	sum := sha256.Sum256([]byte(fullResult))
	resultPath := filepath.Join(config.DataToolResults(), fmt.Sprintf("%x.txt", sum))
	if marker := fmt.Sprintf("[Full result stored at %s]", resultPath); !strings.Contains(receipt, marker) {
		t.Fatalf("receipt missing retrieval marker %q:\n%s", marker, receipt)
	}

	stored, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read persisted result: %v", err)
	}
	if string(stored) != fullResult {
		t.Fatal("persisted result does not exactly match the full tool output")
	}

	dirInfo, err := os.Stat(config.DataToolResults())
	if err != nil {
		t.Fatalf("stat result directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("result directory mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(resultPath)
	if err != nil {
		t.Fatalf("stat result file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("result file mode = %o, want 600", got)
	}
}

func TestRunSingleToolHonorsToolSpecificReceiptLimitBeforeTurnLimit(t *testing.T) {
	tools.ResetBudgetForNewTurn()
	t.Cleanup(tools.ResetBudgetForNewTurn)
	t.Setenv("HOME", t.TempDir())

	fullResult := "HEAD-" + strings.Repeat("bounded-output-", 200) + "-TAIL"
	if len(fullResult) >= tools.DefaultMaxCharsPerTurn {
		t.Fatalf("fixture length = %d, want below turn limit %d", len(fullResult), tools.DefaultMaxCharsPerTurn)
	}

	const toolLimit = 1_024
	boundedTool := tools.NewTool(tools.Tool{
		Name:               "tool_limited_output",
		MaxResultSizeChars: toolLimit,
		Call: func(map[string]any, tools.Context, tools.CanUseToolFn, tools.OnProgress) (tools.ToolResult, error) {
			return tools.ToolResult{Data: fullResult}, nil
		},
		MapResult: func(result any, toolUseID string) types.ToolResultBlock {
			return types.ToolResultBlock{ToolUseID: toolUseID, Content: result.(string)}
		},
	})

	message, err := runSingleTool(
		tools.Context{AbortController: context.Background()},
		types.ToolUseBlock{ID: "tool-limited-1", Name: boundedTool.Name, Input: map[string]any{}},
		types.Message{UUID: "assistant-tool-limited"},
		[]tools.Tool{boundedTool},
		func(string, map[string]any, tools.Context) (tools.PermissionDecision, error) {
			return tools.PermissionDecision{Behavior: tools.Allow}, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("runSingleTool() error = %v", err)
	}

	result := message.Content[0].(types.ToolResultBlock)
	if len(result.Content) > toolLimit {
		t.Fatalf("receipt length = %d, want at most tool limit %d", len(result.Content), toolLimit)
	}
	if !strings.Contains(result.Content, "HEAD-") || !strings.Contains(result.Content, "-TAIL") {
		t.Fatal("tool-limited receipt does not preserve both result ends")
	}
	if !strings.Contains(result.Content, "[Full result stored at ") {
		t.Fatal("tool-limited receipt is missing the durable retrieval marker")
	}
}
