package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *ChatModel) filterSuggestions(input string) []string {
	if input == "" || input == "/" {
		return m.allCommands
	}
	query := strings.ToLower(input)

	var prefixMatches []string
	var containsMatches []string
	var fuzzyMatches []string

	for _, cmd := range m.allCommands {
		lower := strings.ToLower(cmd)
		if strings.HasPrefix(lower, query) {
			prefixMatches = append(prefixMatches, cmd)
		} else if strings.Contains(lower, query) {
			containsMatches = append(containsMatches, cmd)
		} else if fuzzyMatch(query, lower) {
			fuzzyMatches = append(fuzzyMatches, cmd)
		}
	}

	var result []string
	result = append(result, prefixMatches...)
	result = append(result, containsMatches...)
	result = append(result, fuzzyMatches...)
	return result
}

// fuzzyMatch returns true if query approximately matches target using
// subsequence matching with tolerance for small edit distances.
func fuzzyMatch(query, target string) bool {
	if len(query) > len(target) {
		return false
	}
	qi := 0
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if query[qi] == target[ti] {
			qi++
		}
	}
	return qi == len(query)
}

// suggestionMaxVisible is how many suggestion rows show at once: a
// phone pane cannot spare six rows of dropdown over the transcript.
func (m ChatModel) suggestionMaxVisible() int {
	if isMobilePane(m.width) {
		return 3
	}
	return 6
}

// SetModel sets the model name.
func (m *ChatModel) syncSuggestionOffset() {
	maxVisible := m.suggestionMaxVisible()
	if m.suggestionCursor < m.suggestionOffset {
		m.suggestionOffset = m.suggestionCursor
	}
	if m.suggestionCursor >= m.suggestionOffset+maxVisible {
		m.suggestionOffset = m.suggestionCursor - maxVisible + 1
	}
}

// truncateSuggestionDescription clips a command description so the
// suggestion line (prefix + two separator columns + description) stays
// inside the terminal width; an overflowing description is noise.
func (m ChatModel) truncateSuggestionDescription(prefix, description string) string {
	max := m.width - lipgloss.Width(prefix) - 2
	if max < 10 {
		max = 10
	}
	if lipgloss.Width(description) <= max {
		return description
	}
	return description[:max-1] + "…"
}

// renderSuggestions renders the inline suggestion dropdown.
func (m ChatModel) renderSuggestions() string {
	var b strings.Builder
	maxVisible := m.suggestionMaxVisible()
	if len(m.suggestions) < maxVisible {
		maxVisible = len(m.suggestions)
	}

	start := m.suggestionOffset
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > len(m.suggestions) {
		end = len(m.suggestions)
	}

	for i := start; i < end; i++ {
		sug := m.suggestions[i]
		indicator := "  "
		style := HelpDimStyle
		if i == m.suggestionCursor {
			indicator = IndicatorSelected + " "
			style = InfoStyle
		}
		line := style.Render(indicator + sug)
		if description := m.commandDescriptions[sug]; description != "" {
			line += HelpDimStyle.Render("  " + m.truncateSuggestionDescription(indicator+sug, description))
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	if len(m.suggestions) > maxVisible {
		b.WriteString("\n")
		if start > 0 {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  ...%d above", start)))
			if end < len(m.suggestions) {
				b.WriteString(HelpDimStyle.Render(fmt.Sprintf(" | %d below", len(m.suggestions)-end)))
			}
		} else {
			b.WriteString(HelpDimStyle.Render(fmt.Sprintf("  ...and %d more", len(m.suggestions)-end)))
		}
	}
	return b.String()
}

// Focus focuses the chat input.
