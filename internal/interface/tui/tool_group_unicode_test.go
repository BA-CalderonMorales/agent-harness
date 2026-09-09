package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func TestToolGroupRowsRespectUnicodeCellBudget(t *testing.T) {
	m := NewChatModel()
	for _, name := range []string{"bash", "read"} {
		for _, detail := range []string{strings.Repeat("界", 30), strings.Repeat("é", 30), strings.Repeat("e\u0301", 30), strings.Repeat("👩‍💻", 30), "\x1b[31m" + strings.Repeat("界", 30) + "\x1b[0m"} {
			for _, width := range []int{6, 7, 12, 30, 60} {
				rows := m.toolGroupRowsAt(ChatMessage{ToolName: name, ToolDetail: detail}, width)
				for _, row := range rows {
					if !utf8.ValidString(row) || ansi.StringWidth(row) > width-6 {
						t.Fatalf("width=%d row=%q cells=%d", width, row, ansi.StringWidth(row))
					}
				}
			}
		}
	}
}
