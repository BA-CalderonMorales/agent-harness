// Formatted output rendering with professional presentation
// Designed for clarity, scannability, and delight

package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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

// Markers for lists and status
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
	return WarningStyle.Render("⚠ " + message)
}

// RenderInfo renders an info indicator
func RenderInfo(message string) string {
	return InfoStyle.Render("ℹ " + message)
}

// RenderUserInput renders user input in the chat flow
func RenderUserInput(text string) string {
	return fmt.Sprintf("\n%s %s\n", UserStyle.Render("◆"), text)
}

// RenderAgentResponse renders agent response in the chat flow
func RenderAgentResponse(text string) string {
	// Don't add extra newlines if text already has them
	if strings.HasPrefix(text, "\n") {
		return text
	}
	return text
}

// RenderToolUse renders a tool use indicator
func RenderToolUse(toolName, description string) string {
	action := FormatToolAction(toolName, description)
	return fmt.Sprintf("%s %s", DimStyle.Render("→"), DimStyle.Render(action))
}

// RenderToolResult renders a tool result (compact for success, verbose for errors)
func RenderToolResult(success bool, summary string) string {
	if success {
		if summary != "" {
			return fmt.Sprintf("  %s %s", SuccessStyle.Render("✓"), DimStyle.Render(summary))
		}
		return fmt.Sprintf("  %s", SuccessStyle.Render("✓"))
	}
	return fmt.Sprintf("  %s %s", ErrorStyle.Render("✗"), ErrorStyle.Render(summary))
}

// WelcomeScreen renders the contextual welcome screen
func WelcomeScreen(version, model, permissionMode string, gitContext *GitInfo) string {
	var lines []string
	
	// Determine build display
	buildType := "release"
	if gitContext != nil && gitContext.BuildType != "" {
		buildType = gitContext.BuildType
	}
	if strings.Contains(version, "dev") || strings.Contains(version, "local") {
		buildType = "dev"
	}
	
	// Header with persona and version
	lines = append(lines, "")
	versionDisplay := version
	if buildType == "dev" {
		versionDisplay = fmt.Sprintf("%s [dev]", version)
	}
	lines = append(lines, HeaderStyle.Render(fmt.Sprintf("  %s %s", PersonaName, DimStyle.Render(versionDisplay))))
	
	// Context-aware greeting
	greeting := fmt.Sprintf("  %s %s", TimeOfDayGreeting(), GetRandomGreeting())
	lines = append(lines, greeting)
	lines = append(lines, "")
	
	// Compact status line
	statusParts := []string{
		fmt.Sprintf("model: %s", model),
		fmt.Sprintf("permissions: %s", permissionMode),
	}
	
	// Add git context if available
	if gitContext != nil && gitContext.IsRepo {
		gitInfo := gitContext.Branch
		if gitContext.Tag != "" {
			gitInfo = fmt.Sprintf("%s@%s", gitContext.Branch, gitContext.Tag)
		}
		statusParts = append(statusParts, fmt.Sprintf("repo: %s", gitInfo))
	}
	
	lines = append(lines, "  "+DimStyle.Render(strings.Join(statusParts, " • ")))
	lines = append(lines, "")
	
	// Quick hint
	lines = append(lines, DimStyle.Render("  Type /help for commands or just start chatting."))
	lines = append(lines, "")
	
	return strings.Join(lines, "\n")
}

// GitInfo holds git context for rendering
type GitInfo struct {
	IsRepo    bool
	Root      string
	Branch    string
	Tag       string
	BuildType string // "release" or "dev"
}

// RenderStatusReport renders a comprehensive status report
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
	
	// Session stats
	lines = append(lines, RenderField("Session", mode))
	lines = append(lines, RenderField("Messages", fmt.Sprintf("%d", messageCount)))
	lines = append(lines, RenderField("Turns", fmt.Sprintf("%d", turns)))
	lines = append(lines, RenderField("Est. tokens", fmt.Sprintf("%d", estimatedTokens)))
	lines = append(lines, RenderField("Model", model))
	lines = append(lines, "")
	
	// Workspace context
	if projectRoot != "" {
		lines = append(lines, RenderSection("Workspace"))
		lines = append(lines, "")
		lines = append(lines, RenderField("Project", projectRoot))
		if gitBranch != "" {
			lines = append(lines, RenderField("Branch", gitBranch))
		}
		lines = append(lines, "")
	}
	
	// Quick actions
	lines = append(lines, RenderSection("Quick Commands"))
	lines = append(lines, "")
	lines = append(lines, "  /status     Show this report")
	lines = append(lines, "  /compact    Trim session if getting large")
	lines = append(lines, "  /cost       Show token usage and cost")
	lines = append(lines, "  /export     Save conversation to file")
	lines = append(lines, "  /quit       Exit")
	
	return strings.Join(lines, "\n")
}

// RenderCostReport renders cost information
func RenderCostReport(
	inputTokens int,
	outputTokens int,
	cacheCreationInputTokens int,
	cacheReadInputTokens int,
	totalCost float64,
) string {
	var lines []string
	
	lines = append(lines, RenderSection("Usage"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Input tokens", fmt.Sprintf("%d", inputTokens)))
	lines = append(lines, RenderField("Output tokens", fmt.Sprintf("%d", outputTokens)))
	
	if cacheCreationInputTokens > 0 {
		lines = append(lines, RenderField("Cache write", fmt.Sprintf("%d", cacheCreationInputTokens)))
	}
	if cacheReadInputTokens > 0 {
		lines = append(lines, RenderField("Cache read", fmt.Sprintf("%d", cacheReadInputTokens)))
	}
	
	total := inputTokens + outputTokens + cacheReadInputTokens
	lines = append(lines, RenderField("Total", fmt.Sprintf("%d", total)))
	
	if totalCost > 0 {
		lines = append(lines, RenderField("Cost", fmt.Sprintf("$%.4f", totalCost)))
	}
	
	return strings.Join(lines, "\n")
}

// RenderPermissionsReport renders permission settings
func RenderPermissionsReport(currentMode string, modes []struct {
	Name        string
	Description string
	Current     bool
}) string {
	var lines []string
	
	// Current mode description
	var effect string
	for _, m := range modes {
		if m.Current {
			effect = m.Description
			break
		}
	}
	
	lines = append(lines, RenderSection("Permissions"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Mode", currentMode))
	lines = append(lines, RenderField("Effect", effect))
	lines = append(lines, "")
	lines = append(lines, RenderSection("Available Modes"))
	lines = append(lines, "")
	
	for _, mode := range modes {
		marker := AvailableMarker
		if mode.Current {
			marker = CurrentMarker
		}
		lines = append(lines, fmt.Sprintf("  %-16s %s %s",
			LabelStyle.Render(mode.Name),
			marker,
			DimStyle.Render(mode.Description)))
	}
	
	lines = append(lines, "")
	lines = append(lines, DimStyle.Render("  Use /permissions <mode> to switch"))
	
	return strings.Join(lines, "\n")
}

// RenderCompactReport renders compaction results
func RenderCompactReport(removedCount, keptCount int, skipped bool) string {
	var lines []string
	
	lines = append(lines, RenderSection("Session Compacted"))
	lines = append(lines, "")
	
	if skipped {
		lines = append(lines, RenderField("Result", "No compaction needed"))
		lines = append(lines, RenderField("Messages", fmt.Sprintf("%d", keptCount)))
	} else {
		lines = append(lines, RenderField("Removed", fmt.Sprintf("%d messages", removedCount)))
		lines = append(lines, RenderField("Kept", fmt.Sprintf("%d messages", keptCount)))
		lines = append(lines, "")
		lines = append(lines, DimStyle.Render("  Older messages have been summarized to save tokens."))
	}
	
	return strings.Join(lines, "\n")
}

// RenderModelReport renders model information
func RenderModelReport(currentModel string, messageCount int, turns int, aliases map[string]string) string {
	var lines []string
	
	lines = append(lines, RenderSection("Model"))
	lines = append(lines, "")
	lines = append(lines, RenderField("Current", currentModel))
	lines = append(lines, RenderField("Session", fmt.Sprintf("%d messages · %d turns", messageCount, turns)))
	
	if len(aliases) > 0 {
		lines = append(lines, "")
		lines = append(lines, RenderSection("Shortcuts"))
		lines = append(lines, "")
		for alias, model := range aliases {
			lines = append(lines, RenderField(alias, model))
		}
	}
	
	return strings.Join(lines, "\n")
}

// RenderHelp renders the help screen
func RenderHelp(commands map[string]string) string {
	var lines []string
	
	lines = append(lines, "")
	lines = append(lines, HeaderStyle.Render(fmt.Sprintf("  %s Commands", PersonaName)))
	lines = append(lines, "")
	
	// Group commands by category
	categories := map[string][]string{
		"Session": {"/help", "/status", "/clear", "/compact", "/session", "/export"},
		"Settings": {"/model", "/permissions", "/config"},
		"Info": {"/cost", "/diff", "/version"},
		"Exit": {"/quit", "/exit"},
	}
	
	for category, cmds := range categories {
		lines = append(lines, LabelStyle.Render("  "+category))
		for _, cmd := range cmds {
			name := strings.TrimPrefix(cmd, "/")
			if desc, ok := commands[name]; ok {
				lines = append(lines, fmt.Sprintf("    %-12s %s", cmd, DimStyle.Render(desc)))
			}
		}
		lines = append(lines, "")
	}
	
	lines = append(lines, DimStyle.Render("  Pro tip: Type just the first few letters of a command and press Tab."))
	lines = append(lines, "")
	
	return strings.Join(lines, "\n")
}

// RenderGoodbye renders the exit message
func RenderGoodbye(costSummary string) string {
	var lines []string
	
	lines = append(lines, "")
	lines = append(lines, DimStyle.Render("  Goodbye. "+costSummary))
	lines = append(lines, "")
	
	return strings.Join(lines, "\n")
}

// RenderSuggestion renders a contextual suggestion
func RenderSuggestion(text string) string {
	return fmt.Sprintf("  %s %s", DimStyle.Render("Tip:"), DimStyle.Render(text))
}

// RenderAutoSave renders an auto-save notification
func RenderAutoSave(path string) string {
	return DimStyle.Render(fmt.Sprintf("  (Auto-saved to %s)", path))
}

// RenderSeparator renders a visual separator
func RenderSeparator() string {
	width := 40
	if w, _, err := GetTerminalSize(); err == nil && w > 0 {
		width = w / 2
		if width > 60 {
			width = 60
		}
	}
	return DimStyle.Render(strings.Repeat("─", width))
}

// RenderConversationTurn renders a complete conversation turn
func RenderConversationTurn(userInput, agentResponse string, toolsUsed []string) string {
	var lines []string
	
	// User input
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

// Spinner provides animated progress indication
type Spinner struct {
	frames []string
	index  int
}

// NewSpinner creates a new spinner with Unicode frames
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:  0,
	}
}

// NewSimpleSpinner creates a spinner with simple ASCII frames
func NewSimpleSpinner() *Spinner {
	return &Spinner{
		frames: []string{"|", "/", "-", "\\"},
		index:  0,
	}
}

// Next returns the next animation frame
func (s *Spinner) Next() string {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	return frame
}

// Current returns the current frame without advancing
func (s *Spinner) Current() string {
	return s.frames[s.index]
}

// ProgressBar renders a progress bar
func ProgressBar(current, total int, width int) string {
	if total <= 0 {
		return ""
	}
	
	filled := int(float64(current) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
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

// TruncateMiddle truncates in the middle (good for paths)
func TruncateMiddle(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return "..."
	}
	
	sideLen := (maxLen - 3) / 2
	return text[:sideLen] + "..." + text[len(text)-sideLen:]
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

// FormatDuration formats a duration in a human-friendly way
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
	}
	return fmt.Sprintf("%.1fm", float64(d)/float64(time.Minute))
}

// FormatBytes formats byte size in a human-friendly way
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// DetectTermux detects if running in Termux environment
func DetectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" || 
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// UseSimpleRendering returns true if we should use simple ASCII rendering
func UseSimpleRendering() bool {
	return DetectTermux() || os.Getenv("AGENT_HARNESS_SIMPLE_UI") != ""
}
