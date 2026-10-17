package types

import "testing"

func TestContentBlockRejectsMalformedFields(t *testing.T) {
	for _, input := range []string{
		`[`,
		`{"thinking":7}`,
		`{"tool_use_id":7}`,
		`{"name":7}`,
		`{"text":7}`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := unmarshalContentBlock([]byte(input)); err == nil {
				t.Fatal("malformed content must return an error")
			}
		})
	}
}

func TestContentBlockRecognizesEachToolUseField(t *testing.T) {
	for _, input := range []string{
		`{"id":"call-1"}`,
		`{"name":"read"}`,
		`{"input":{"path":"file.txt"}}`,
	} {
		t.Run(input, func(t *testing.T) {
			block, err := unmarshalContentBlock([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := block.(ToolUseBlock); !ok {
				t.Fatalf("got %T, want ToolUseBlock", block)
			}
		})
	}
}
