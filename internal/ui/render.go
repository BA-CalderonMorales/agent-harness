// Formatted output rendering inspired by claw-code

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles for different output elements
var (
	// Header style for section headers
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	// Label style for field labels
	LabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	// Value style for field values
	ValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	// Success style for success messages
	SuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ade80"))

	// Error style for error messages
	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f87171"))

	// Warning style for warnings
	WarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24"))

	// Info style for info messages
	InfoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60a5fa"))

	// Dim style for dimmed text
	DimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	// CurrentMarker for current selection
	CurrentMarker = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3b82f6")).
		Bold(true).
		Render("●")

	// AvailableMarker for available options
	AvailableMarker = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Render("○")
)

// RenderField renders a labeled field
func RenderField(label, value string) string {
	return fmt.Sprintf("  %s %s",
		LabelStyle.Render(fmt.Sprintf("%-16s", label)),
		ValueStyle.Render(value))
}

// RenderSection renders a section header
func RenderSection(title string) string {
	return HeaderStyle.Render(title)
}

// RenderSuccess renders a success message
func RenderSuccess(message string) string {
	return SuccessStyle.Render("✓ " + message)
}

// RenderError renders an error message
func RenderError(message string) string {
	return ErrorStyle.Render("✗ " + message)
}

// RenderWarning renders a warning message
func RenderWarning(message string) string {
	return WarningStyle.Render("⚠ " + message)
}

// RenderInfo renders an info message
func RenderInfo(message string) string {
	return InfoStyle.Render("ℹ " + message)
}

// RenderStatusReport renders a status report similar to claw-code
func RenderStatusReport(
	mode string,
	messageCount int,
	turns int,
	estimatedTokens int,
	model string,
	projectRoot string,
	gitBranch string,
) string {
	var lines []string

	lines = append(lines, RenderSection("Status"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Session mode", mode))
	lines = append(lines, RenderField("Messages", fmt.Sprintf("%d", messageCount)))
	lines = append(lines, RenderField("Turns", fmt.Sprintf("%d", turns)))
	lines = append(lines, RenderField("Est. tokens", fmt.Sprintf("%d", estimatedTokens)))
	lines = append(lines, RenderField("Model", model))
	lines = append(lines, "")

	if projectRoot != "" {
		lines = append(lines, RenderSection("Workspace"))
		lines = append(lines, "")
		lines = append(lines, RenderField("Project root", projectRoot))
		if gitBranch != "" {
			lines = append(lines, RenderField("Git branch", gitBranch))
		}
		lines = append(lines, "")
	}

	lines = append(lines, RenderSection("Next"))
	lines = append(lines, "")
	lines = append(lines, "  /status     Show this status report")
	lines = append(lines, "  /compact    Trim session if getting large")
	lines = append(lines, "  /cost       Show token usage and cost")
	lines = append(lines, "  /export     Export conversation to file")

	return strings.Join(lines, "\n")
}

// RenderCostReport renders a cost report
func RenderCostReport(
	inputTokens int,
	outputTokens int,
	cacheCreationInputTokens int,
	cacheReadInputTokens int,
	totalCost float64,
) string {
	var lines []string

	lines = append(lines, RenderSection("Cost"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Input tokens", fmt.Sprintf("%d", inputTokens)))
	lines = append(lines, RenderField("Output tokens", fmt.Sprintf("%d", outputTokens)))
	if cacheCreationInputTokens > 0 {
		lines = append(lines, RenderField("Cache create", fmt.Sprintf("%d", cacheCreationInputTokens)))
	}
	if cacheReadInputTokens > 0 {
		lines = append(lines, RenderField("Cache read", fmt.Sprintf("%d", cacheReadInputTokens)))
	}
	totalTokens := inputTokens + outputTokens + cacheReadInputTokens
	lines = append(lines, RenderField("Total tokens", fmt.Sprintf("%d", totalTokens)))
	if totalCost > 0 {
		lines = append(lines, RenderField("Total cost", fmt.Sprintf("$%.4f", totalCost)))
	}
	lines = append(lines, "")
	lines = append(lines, RenderSection("Next"))
	lines = append(lines, "")
	lines = append(lines, "  /status     See session + workspace context")
	lines = append(lines, "  /compact    Trim local history if the session is getting large")

	return strings.Join(lines, "\n")
}

// RenderPermissionsReport renders a permissions report
func RenderPermissionsReport(currentMode string, modes []struct {
	Name        string
	Description string
	Current     bool
}) string {
	var lines []string

	// Find current mode description
	var effect string
	for _, m := range modes {
		if m.Current {
			effect = m.Description
			break
		}
	}

	lines = append(lines, RenderSection("Permissions"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Active mode", currentMode))
	lines = append(lines, RenderField("Effect", effect))
	lines = append(lines, "")
	lines = append(lines, RenderSection("Modes"))
	lines = append(lines, "")

	for _, mode := range modes {
		marker := AvailableMarker
		if mode.Current {
			marker = CurrentMarker
		}
		lines = append(lines, fmt.Sprintf("  %-18s %s %s",
			LabelStyle.Render(mode.Name),
			marker,
			DimStyle.Render(mode.Description)))
	}

	lines = append(lines, "")
	lines = append(lines, RenderSection("Next"))
	lines = append(lines, "")
	lines = append(lines, "  /permissions              Show the current mode")
	lines = append(lines, "  /permissions <mode>       Switch modes for subsequent tool calls")

	return strings.Join(lines, "\n")
}

// RenderCompactReport renders a compaction report
func RenderCompactReport(removedCount, keptCount int, skipped bool) string {
	var lines []string

	lines = append(lines, RenderSection("Compact"))
	lines = append(lines, "")

	if skipped {
		lines = append(lines, RenderField("Result", "skipped"))
		lines = append(lines, RenderField("Reason", "Session is already below the compaction threshold"))
		lines = append(lines, RenderField("Messages kept", fmt.Sprintf("%d", keptCount)))
	} else {
		lines = append(lines, RenderField("Result", "compacted"))
		lines = append(lines, RenderField("Messages removed", fmt.Sprintf("%d", removedCount)))
		lines = append(lines, RenderField("Messages kept", fmt.Sprintf("%d", keptCount)))
		lines = append(lines, "")
		lines = append(lines, DimStyle.Render("  Tip: Use /status to review the trimmed session"))
	}

	return strings.Join(lines, "\n")
}

// RenderModelReport renders a model report
func RenderModelReport(currentModel string, messageCount int, turns int, aliases map[string]string) string {
	var lines []string

	lines = append(lines, RenderSection("Model"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Current", currentModel))
	lines = append(lines, RenderField("Session", fmt.Sprintf("%d messages · %d turns", messageCount, turns)))

	if len(aliases) > 0 {
		lines = append(lines, "")
		lines = append(lines, RenderSection("Aliases"))
		lines = append(lines, "")
		for alias, model := range aliases {
			lines = append(lines, RenderField(alias, model))
		}
	}

	lines = append(lines, "")
	lines = append(lines, RenderSection("Next"))
	lines = append(lines, "")
	lines = append(lines, "  /model           Show the current model")
	lines = append(lines, "  /model <name>    Switch models for this session")

	return strings.Join(lines, "\n")
}

// RenderSessionReport renders a session report
func RenderSessionReport(sessionID string, createdAt, updatedAt string, messageCount, turns int, model string) string {
	var lines []string

	lines = append(lines, RenderSection("Session"))
	lines = append(lines, "")
	lines = append(lines, RenderField("ID", sessionID[:8]))
	lines = append(lines, RenderField("Created", createdAt))
	lines = append(lines, RenderField("Updated", updatedAt))
	lines = append(lines, RenderField("Messages", fmt.Sprintf("%d", messageCount)))
	lines = append(lines, RenderField("Turns", fmt.Sprintf("%d", turns)))
	lines = append(lines, RenderField("Model", model))

	return strings.Join(lines, "\n")
}

// RenderHelp renders the help text
func RenderHelp(commands map[string]string) string {
	var lines []string

	lines = append(lines, RenderSection("Available commands"))
	lines = append(lines, "")

	// Group commands
	categories := map[string][]string{
		"Session":  {"/help", "/status", "/clear", "/compact", "/session"},
		"Settings": {"/model", "/permissions", "/config", "/memory"},
		"Output":   {"/cost", "/diff", "/export", "/version"},
	}

	for category, cmds := range categories {
		lines = append(lines, LabelStyle.Render(category+":"))
		for _, cmd := range cmds {
			name := strings.TrimPrefix(cmd, "/")
			if desc, ok := commands[name]; ok {
				lines = append(lines, fmt.Sprintf("  %-15s %s", cmd, DimStyle.Render(desc)))
			}
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// Spinner represents a loading spinner
type Spinner struct {
	frames []string
	index  int
}

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:  0,
	}
}

// Next returns the next frame
func (s *Spinner) Next() string {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	return frame
}

// RenderSpinner renders a spinner with a message
func RenderSpinner(spinner *Spinner, message string) string {
	return fmt.Sprintf("%s %s", spinner.Next(), DimStyle.Render(message))
}

// ProgressBar renders a simple progress bar
func ProgressBar(current, total int, width int) string {
	if total <= 0 {
		return ""
	}

	filled := int(float64(current) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percent := int(float64(current) / float64(total) * 100)

	return fmt.Sprintf("[%s] %d%%", bar, percent)
}

// Truncate truncates text with ellipsis
func Truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

// WordWrap wraps text to a maximum width
func WordWrap(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)
	return lines
}
