package builtin

import (
	"strings"
	"testing"
)

// The export tool keeps its own copy of the redaction rules, separate
// from the session exporter. These tests keep the copy honest: a header
// map key or a JSON-formatted header must not survive either path.
func TestExportRedactTreatsHeaderKeysAsSensitive(t *testing.T) {
	got := redactExportMap(map[string]any{
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
	got := redactExportString(text)
	for _, leak := range []string{"opaque-value", "dummy"} {
		if strings.Contains(got, leak) {
			t.Fatalf("redactExportString() = %q, still contains %q", got, leak)
		}
	}
}
