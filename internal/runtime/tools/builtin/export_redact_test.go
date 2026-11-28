package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/redact"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestExportRedactTreatsHeaderKeysAsSensitive(t *testing.T) {
	got := redact.Map(map[string]any{
		"Authorization": "Bearer opaque-value",
		"x-api-key":     "dummy",
		"Cookie":        "session=abc123",
		"nested":        map[string]any{"x-auth-token": "tok"},
		"safe":          "keep",
	})
	for _, key := range []string{"Authorization", "x-api-key", "Cookie"} {
		if got[key] != "<redacted>" {
			t.Fatalf("%s = %#v, want <redacted>", key, got[key])
		}
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok || nested["x-auth-token"] != "<redacted>" {
		t.Fatalf("nested = %#v, want x-auth-token redacted", got["nested"])
	}
	if got["safe"] != "keep" {
		t.Fatalf("safe = %#v, want ordinary values preserved", got["safe"])
	}
}

func TestExportRedactJSONHeaderText(t *testing.T) {
	text := `{"Authorization":"Bearer opaque-value","x-api-key":"dummy"}`
	got := redact.String(text)
	for _, leak := range []string{"opaque-value", "dummy"} {
		if strings.Contains(got, leak) {
			t.Fatalf("redact.String() = %q, still contains %q", got, leak)
		}
	}
}

func TestExportToolJSONPreservesRedactedContent(t *testing.T) {
	redact.SetSecret("exact-live-secret")
	defer redact.SetSecret("")

	outPath := filepath.Join(t.TempDir(), "transcript.json")
	ctx := tools.Context{
		Messages: []types.Message{
			{
				Role: types.RoleUser,
				Content: []types.ContentBlock{
					types.TextBlock{Text: "hello exact-live-secret world"},
				},
			},
			{
				Role: types.RoleAssistant,
				Content: []types.ContentBlock{
					types.ToolUseBlock{
						ID:    "t1",
						Name:  "bash",
						Input: map[string]any{"command": "echo hi", "api_key": "secret-val"},
					},
				},
			},
		},
	}

	if _, err := ExportTool.Call(map[string]any{"file_path": outPath, "format": "json"}, ctx, nil, nil); err != nil {
		t.Fatalf("ExportTool.Call() error = %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "hello <redacted> world") {
		t.Fatalf("JSON export dropped or failed to redact user text: %s", body)
	}
	if !strings.Contains(body, `"command": "echo hi"`) {
		t.Fatalf("JSON export dropped tool input: %s", body)
	}
	if strings.Contains(body, "exact-live-secret") || strings.Contains(body, "secret-val") {
		t.Fatalf("JSON export leaked secret: %s", body)
	}
}
