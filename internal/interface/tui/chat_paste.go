package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/pkg/format"
)

// PasteCollapseThreshold is how large a bracketed paste must be before
// the composer collapses it to a placeholder token: pasting is a
// transfer of material, not an editing session — nobody wants 8k
// characters of log file scrolling past in their input box.
const PasteCollapseThreshold = 300

// PastePreviewLines is how many leading lines of a collapsed paste the
// transcript shows verbatim: the head of the content identifies what
// was sent, and a marker states exactly how much was hidden.
const PastePreviewLines = 8

// pastePreview renders pasted input for the transcript. A blackout
// marker ("[Pasted text, N characters]") hides the very content an
// audit needs; a bounded preview keeps the transcript honest without
// flooding it.
func pastePreview(input string) string {
	lines := strings.Split(input, "\n")
	head := lines
	if len(lines) > PastePreviewLines {
		head = lines[:PastePreviewLines]
	}
	text := strings.Join(head, "\n")
	if hidden := len(lines) - len(head); hidden > 0 {
		text += fmt.Sprintf("\n[… +%d more lines · %d characters total]", hidden, len(input))
	} else if len(input) > PasteDisplayThreshold {
		text += fmt.Sprintf(" [… %d characters total]", len(input))
	}
	return text
}

// stashPaste parks a large paste and returns its placeholder token.
// The full content rides in pendingPastes until submit expands it.
func (m *ChatModel) stashPaste(content string) (token string, full string) {
	if m.pendingPastes == nil {
		m.pendingPastes = make(map[string]string)
	}
	n := len(m.pendingPastes) + 1
	for {
		token = fmt.Sprintf("[paste #%d · %s]", n, format.HumanBytes(int64(len(content))))
		if _, taken := m.pendingPastes[token]; !taken {
			break
		}
		n++
	}
	m.pendingPastes[token] = content
	return token, content
}

// expandPasteTokens restores stashed paste content on submit. Unknown
// tokens (user-typed text that merely looks like one) pass through
// untouched.
func (m *ChatModel) expandPasteTokens(text string) string {
	for token, content := range m.pendingPastes {
		text = strings.ReplaceAll(text, token, content)
	}
	return text
}

// clearPendingPastes drops stashed pastes — the draft was abandoned.
func (m *ChatModel) clearPendingPastes() {
	m.pendingPastes = nil
}

// pasteFeedback surfaces the collapse without stealing focus: a
// transient status now, the token visible in the composer.
func (m *ChatModel) pasteFeedback(token, content string) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{
			Text: fmt.Sprintf("Pasted %s (collapsed as %s)", format.HumanBytes(int64(len(content))), token),
			Type: "info",
		}
	}
}
