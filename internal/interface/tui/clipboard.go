package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Clipboard: the TUI captures the mouse (scroll + click targeting), so
// the terminal's own text selection fights the UI. 'y' in normal mode
// copies a specific record instead — the expanded one when something is
// expanded, otherwise the latest assistant reply. Delivery tries the
// platform clipboard first and falls back to OSC 52, which modern
// terminals (Windows Terminal, kitty, WezTerm, alacritty) honor even
// over SSH.

// copyToClipboard delivers text to the system clipboard. It reports
// which transport carried the text ("clipboard" via tool or terminal).
func copyToClipboard(text string) bool {
	for _, argv := range clipboardCommands() {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return writeOSC52(text)
}

// clipboardCommands lists platform clipboard writers in preference
// order. clip.exe comes first: WSL always has it, and it talks straight
// to the Windows clipboard.
func clipboardCommands() [][]string {
	if runtime.GOOS == "windows" {
		return [][]string{{"clip"}}
	}
	commands := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"pbcopy"},
	}
	// WSL: clip.exe lives on the Windows PATH, reachable via /mnt/c.
	if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		commands = append([][]string{{"clip.exe"}}, commands...)
	}
	return commands
}

// writeOSC52 sends the clipboard escape sequence directly to the
// terminal. Single atomic write: the renderer races raw stdout writes,
// so the sequence must land in one syscall.
func writeOSC52(text string) bool {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := os.Stdout.WriteString("\x1b]52;c;" + encoded + "\x1b\\")
	return err == nil
}

// CopyRecord returns the text a 'y' should copy: the expanded record's
// content when one is open (that is the specific thing the user is
// looking at), otherwise the latest assistant reply. The second return
// describes what was copied for the status flash.
func (m *ChatModel) CopyRecord() (string, string) {
	if m.expandedMessageID != "" {
		for i := range m.messages {
			if m.messages[i].ID == m.expandedMessageID {
				msg := m.messages[i]
				content := msg.Content
				if msg.IsTool && msg.ToolDetail != "" {
					content = msg.ToolDetail + "\n" + msg.Content
				}
				return content, recordLabel(msg)
			}
		}
	}
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" && strings.TrimSpace(m.messages[i].Content) != "" {
			return m.messages[i].Content, "latest reply"
		}
	}
	return "", ""
}

func recordLabel(msg ChatMessage) string {
	if msg.IsTool {
		return fmt.Sprintf("%s record", msg.ToolDisplayName)
	}
	return "expanded record"
}

// CopyConversation returns the entire transcript in order, one labeled
// block per message, ready for pasting into a bug report or a follow-up
// chat. The count of copied messages comes back for the status flash.
func (m *ChatModel) CopyConversation() (string, int) {
	if len(m.messages) == 0 {
		return "", 0
	}
	var b strings.Builder
	n := 0
	for _, msg := range m.messages {
		content := strings.TrimSpace(msg.Content)
		if msg.IsTool && msg.ToolDetail != "" {
			content = strings.TrimSpace(msg.ToolDetail) + "\n" + content
		}
		if content == "" {
			continue
		}
		fmt.Fprintf(&b, "[%s]\n%s\n\n", msg.Role, content)
		n++
	}
	return strings.TrimRight(b.String(), "\n"), n
}
