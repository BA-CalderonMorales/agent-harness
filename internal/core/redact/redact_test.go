package redact

import (
	"strings"
	"sync"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestMessagesPreservesContentAndRedactsSecrets(t *testing.T) {
	SetSecret("custom-opaque-key-999")
	defer SetSecret("")

	orig := []types.Message{
		{
			Role: types.RoleUser,
			Content: []types.ContentBlock{
				types.TextBlock{Text: "use custom-opaque-key-999 at /home/alice/repo"},
			},
		},
		{
			Role: types.RoleAssistant,
			Content: []types.ContentBlock{
				types.ThinkingBlock{Thinking: "secret thought", Signature: "sig123"},
				types.ToolUseBlock{
					ID:   "call_1",
					Name: "web_fetch",
					Input: map[string]any{
						"Authorization": "Bearer live-tok",
						"url":           "https://example.com",
					},
				},
			},
		},
		{
			Role: types.RoleUser,
			Content: []types.ContentBlock{
				types.ToolResultBlock{
					ToolUseID: "call_1",
					Content:   "x-api-key: top-secret-val",
				},
			},
		},
	}

	got := Messages(orig)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if len(got[0].Content) != 1 || len(got[1].Content) != 2 || len(got[2].Content) != 1 {
		t.Fatalf("content lengths = [%d, %d, %d], want [1, 2, 1]",
			len(got[0].Content), len(got[1].Content), len(got[2].Content))
	}

	tb := got[0].Content[0].(types.TextBlock)
	if strings.Contains(tb.Text, "custom-opaque-key-999") || strings.Contains(tb.Text, "/home/alice") {
		t.Fatalf("user text leaked secret/path: %q", tb.Text)
	}
	// Original input must stay unmutated.
	if !strings.Contains(orig[0].Content[0].(types.TextBlock).Text, "custom-opaque-key-999") {
		t.Fatal("Messages mutated input slice")
	}

	th := got[1].Content[0].(types.ThinkingBlock)
	if th.Thinking != "<redacted>" || th.Signature != "<redacted>" {
		t.Fatalf("thinking not redacted: %+v", th)
	}
	tu := got[1].Content[1].(types.ToolUseBlock)
	if tu.Input["Authorization"] != "<redacted>" || tu.Input["url"] != "https://example.com" {
		t.Fatalf("tool input unexpected: %#v", tu.Input)
	}
	tr := got[2].Content[0].(types.ToolResultBlock)
	if strings.Contains(tr.Content, "top-secret-val") {
		t.Fatalf("tool result leaked secret: %q", tr.Content)
	}
}

func TestStringRedactsQuotedCredentialKeys(t *testing.T) {
	SetSecret("")
	input := `{"password":"super-secret-value","api_key": "live key value","user":"alice"}`
	got := String(input)
	if strings.Contains(got, "super-secret-value") || strings.Contains(got, "live key value") {
		t.Fatalf("quoted credential values leaked: %q", got)
	}
	if !strings.Contains(got, `"user":"alice"`) {
		t.Fatalf("ordinary JSON field corrupted: %q", got)
	}
	plain := "Please enter your password before proceeding."
	if String(plain) != plain {
		t.Fatalf("ordinary prose altered: %q", String(plain))
	}
}

func TestValueRedactsTypedContainersWithoutMutatingInput(t *testing.T) {
	SetSecret("opaque-live-value")
	defer SetSecret("")

	origSlice := []string{"opaque-live-value", "keep-me"}
	origMap := map[string]string{"password": "pw-value", "note": "has opaque-live-value"}
	nested := []map[string]string{{"token": "tok", "msg": "opaque-live-value"}}

	redacted := Map(map[string]any{
		"args":   origSlice,
		"meta":   origMap,
		"nested": nested,
		"count":  42,
	})

	gotSlice := redacted["args"].([]string)
	if gotSlice[0] != "<redacted>" || gotSlice[1] != "keep-me" {
		t.Fatalf("redacted []string = %#v", gotSlice)
	}
	if origSlice[0] != "opaque-live-value" {
		t.Fatal("Map mutated input []string")
	}

	gotMap := redacted["meta"].(map[string]string)
	if gotMap["password"] != "<redacted>" || strings.Contains(gotMap["note"], "opaque-live-value") {
		t.Fatalf("redacted map[string]string = %#v", gotMap)
	}
	if origMap["password"] != "pw-value" {
		t.Fatal("Map mutated input map[string]string")
	}

	gotNested := redacted["nested"].([]any)[0].(map[string]string)
	if gotNested["token"] != "<redacted>" || strings.Contains(gotNested["msg"], "opaque-live-value") {
		t.Fatalf("redacted nested typed container = %#v", gotNested)
	}
	if redacted["count"] != 42 {
		t.Fatalf("ordinary scalar altered: %#v", redacted["count"])
	}
}

func TestMessagesRedactsPointerContentBlocksAndSkipsNilPointers(t *testing.T) {
	SetSecret("ptr-secret-value")
	defer SetSecret("")

	origText := &types.TextBlock{Text: "leak ptr-secret-value"}
	origThink := &types.ThinkingBlock{Thinking: "private reasoning", Signature: "sig"}
	origUse := &types.ToolUseBlock{ID: "c1", Name: "fetch", Input: map[string]any{"api_key": "k", "note": "ptr-secret-value"}}
	origRes := &types.ToolResultBlock{ToolUseID: "c1", Content: "ptr-secret-value"}
	var nilText *types.TextBlock

	msgs := []types.Message{{
		Role:    types.RoleAssistant,
		Content: []types.ContentBlock{origText, origThink, origUse, origRes, nilText},
	}}
	got := Messages(msgs)
	if len(got[0].Content) != 4 {
		t.Fatalf("len(Content) = %d, want 4 (nil pointer skipped)", len(got[0].Content))
	}
	if tb := got[0].Content[0].(*types.TextBlock); strings.Contains(tb.Text, "ptr-secret-value") {
		t.Fatalf("*TextBlock leaked secret: %q", tb.Text)
	}
	if !strings.Contains(origText.Text, "ptr-secret-value") {
		t.Fatal("Messages mutated input *TextBlock")
	}
	if th := got[0].Content[1].(*types.ThinkingBlock); th.Thinking != "<redacted>" || th.Signature != "<redacted>" {
		t.Fatalf("*ThinkingBlock not redacted: %+v", th)
	}
	if tu := got[0].Content[2].(*types.ToolUseBlock); tu.Input["api_key"] != "<redacted>" || strings.Contains(tu.Input["note"].(string), "ptr-secret-value") {
		t.Fatalf("*ToolUseBlock not redacted: %#v", tu.Input)
	}
	if tr := got[0].Content[3].(*types.ToolResultBlock); strings.Contains(tr.Content, "ptr-secret-value") {
		t.Fatalf("*ToolResultBlock leaked secret: %q", tr.Content)
	}
}

func TestSetSecretConcurrentSafety(t *testing.T) {
	defer SetSecret("")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				SetSecret("secret-token-xyz")
				SetSecret("")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = String("hello secret-token-xyz world")
			}
		}()
	}
	wg.Wait()
}
