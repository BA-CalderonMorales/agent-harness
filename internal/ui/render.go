// Formatted output rendering with professional presentation
// Clean visual language: ◆ → ✓ ✗ (no emojis)

package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// Base styles
var (
	// Primary styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff"))

	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff"))

	// Semantic styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ade80"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f87171"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbbf24"))

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60a5fa"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	// Interactive styles
	UserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a78bfa")).
			Bold(true)

	AgentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ade80"))

	ToolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fbbf24")).
			Italic(true)
)

// Markers for lists and status - ASCII only, no emojis
var (
	CurrentMarker   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b82f6")).Bold(true).Render("●")
	AvailableMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("○")
	BulletMarker    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("•")
	ArrowMarker     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("→")
)

// RenderField renders a labeled field with consistent alignment
func RenderField(label, value string) string {
	return fmt.Sprintf("  %s %s",
		LabelStyle.Render(fmt.Sprintf("%-14s", label)),
		ValueStyle.Render(value))
}

// RenderCompactField renders a compact field for dense displays
func RenderCompactField(label, value string) string {
	return fmt.Sprintf("%s %s",
		DimStyle.Render(label+":"),
		value)
}

// RenderSection renders a section header
func RenderSection(title string) string {
	return HeaderStyle.Render(title)
}

// RenderSuccess renders a success indicator
func RenderSuccess(message string) string {
	return SuccessStyle.Render("✓ " + message)
}

// RenderError renders an error indicator
func RenderError(message string) string {
	return ErrorStyle.Render("✗ " + message)
}

// RenderWarning renders a warning indicator
func RenderWarning(message string) string {
	return WarningStyle.Render("! " + message)
}

// RenderInfo renders an info indicator
func RenderInfo(message string) string {
	return InfoStyle.Render("i " + message)
}

// RenderUserInput renders user input with diamond indicator
func RenderUserInput(text string) string {
	return fmt.Sprintf("\n◆ %s\n", text)
}

// RenderAgentResponse renders agent response
func RenderAgentResponse(text string) string {
	if strings.HasPrefix(text, "\n") {
		return text
	}
	return text
}

// RenderToolUse renders a tool use indicator
func RenderToolUse(toolName, description string) string {
	action := FormatToolAction(toolName, description)
	return fmt.Sprintf("→ %s", DimStyle.Render(action))
}

// RenderToolResult renders a tool result
func RenderToolResult(success bool, summary string) string {
	if success {
		if summary != "" {
			return fmt.Sprintf("  ✓ %s", DimStyle.Render(summary))
		}
		return fmt.Sprintf("  %s", SuccessStyle.Render("✓"))
	}
	return fmt.Sprintf("  ✗ %s", ErrorStyle.Render(summary))
}
func RenderConversationTurn(userInput, agentResponse string, toolsUsed []string) string {
	var lines []string

	// User input with diamond
	lines = append(lines, RenderUserInput(userInput))

	// Tool uses (if any)
	for _, tool := range toolsUsed {
		lines = append(lines, "  "+RenderToolUse(tool, ""))
	}

	// Agent response
	lines = append(lines, RenderAgentResponse(agentResponse))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}
