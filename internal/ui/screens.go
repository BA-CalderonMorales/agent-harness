package ui

import (
	"fmt"
	"strings"
)

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
	BuildType string
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
		"Session":  {"/help", "/status", "/clear", "/compact", "/session", "/export"},
		"Settings": {"/model", "/permissions", "/config"},
		"Info":     {"/cost", "/diff", "/version"},
		"Exit":     {"/quit", "/exit"},
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
	return fmt.Sprintf("  tip: %s", DimStyle.Render(text))
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
