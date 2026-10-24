package tui

import (
	"strings"
	"testing"
)

// During a turn the composer has to say what typing does: queueing a
// message for later, or redirecting the turn that is running now.

func TestComposerNamesTheSteerAffordanceDuringATurn(t *testing.T) {
	var m ChatModel
	m.focused = true
	m.thinking = true
	m.SyncPlaceholder()

	if !strings.Contains(m.textarea.Placeholder, "/steer") {
		t.Fatalf("placeholder = %q, want the steer affordance", m.textarea.Placeholder)
	}
	if !strings.Contains(m.textarea.Placeholder, "queue") {
		t.Fatalf("placeholder = %q, want to say that typing queues", m.textarea.Placeholder)
	}
}

func TestComposerIsPlainWhenNoTurnIsRunning(t *testing.T) {
	var m ChatModel
	m.focused = true
	m.SyncPlaceholder()

	if m.textarea.Placeholder != "Type a message..." {
		t.Fatalf("placeholder = %q, want the plain invitation", m.textarea.Placeholder)
	}
}

func TestComposerInNavigateModeAdvertisesTheKeys(t *testing.T) {
	var m ChatModel
	m.focused = false
	m.thinking = true
	m.SyncPlaceholder()

	if !strings.Contains(m.textarea.Placeholder, `"i" to type`) {
		t.Fatalf("placeholder = %q, want the navigate-mode hint", m.textarea.Placeholder)
	}
}
