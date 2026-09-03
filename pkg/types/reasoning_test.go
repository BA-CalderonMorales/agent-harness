package types
import "testing"
// P2-12: reasoning_content field present in ThinkingBlock (from message.go)
func TestReasoningContentFieldExists(t *testing.T) {
	b := ThinkingBlock{Thinking: "hello", Signature: "sig-1"}
	if b.Thinking != "hello" {
		t.Fatalf("thinking content missing: %v", b)
	}
}
