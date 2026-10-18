package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextContentMetadataIsNotFlattened(t *testing.T) {
	var input CallToolResult
	wire := `{"content":[{"type":"text","text":"target bytes","_meta":{"provenance":{"untrusted_content":true}},"annotations":{"audience":["assistant"],"priority":0.5}}]}`
	if err := json.Unmarshal([]byte(wire), &input); err != nil {
		t.Fatal(err)
	}
	got, err := renderToolResult(&input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, `"untrusted_content":true`) || !strings.Contains(got.Content, `"annotations"`) {
		t.Fatal("plain-text shortcut discarded block-level provenance/annotations")
	}
}

func TestResultPreservesStructuredOnlyMediaResourcesAndErrorMetadata(t *testing.T) {
	for _, wire := range []string{
		`{"content":[],"structuredContent":{"evidence_id":"structured-only"}}`,
		`{"content":[{"type":"image","data":"cGl4ZWxz","mimeType":"image/png"}]}`,
		`{"content":[{"type":"resource_link","uri":"file:///evidence","name":"artifact"}]}`,
		`{"content":[{"type":"resource","resource":{"uri":"file:///evidence","text":"source bytes","mimeType":"text/plain"}}]}`,
		`{"content":[{"type":"text","text":"retry with a readable target"}],"isError":true,"_meta":{"provenance":{"untrusted_content":true}}}`,
	} {
		t.Run(wire, func(t *testing.T) {
			var input CallToolResult
			if err := json.Unmarshal([]byte(wire), &input); err != nil {
				t.Fatal(err)
			}
			out, err := renderToolResult(&input)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			canonical, err := json.Marshal(&input)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(canonical, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(out.Content), &got); err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			if string(gotJSON) != string(wantJSON) || out.IsError != input.IsError {
				t.Fatal("MCP result lost content or outcome")
			}
		})
	}
}

func TestResultSeparatesTextBlocksAndRejectsMissingResult(t *testing.T) {
	var input CallToolResult
	if err := json.Unmarshal([]byte(`{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	got, err := renderToolResult(&input)
	if err != nil || got.Content != "first\nsecond" || got.IsError {
		t.Fatalf("ambiguous text blocks: %+v, %v", got, err)
	}
	if _, err := renderToolResult(nil); err == nil || !strings.Contains(err.Error(), "no tool result") {
		t.Fatal("missing result became success")
	}
}
