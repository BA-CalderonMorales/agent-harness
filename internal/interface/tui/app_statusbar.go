package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"os"
	"path/filepath"
	"strings"
)

func (a App) renderStatusBar() string {
	if a.statusMessage != "" {
		var style lipgloss.Style
		switch a.statusType {
		case "success":
			style = SuccessStyle
		case "error":
			style = ErrorStyle
		case "warning":
			style = WarningStyle
		default:
			style = InfoStyle
		}
		content := " " + style.Render(a.statusMessage)
		return StatusBarStyle.Width(a.width).PaddingBottom(1).PaddingLeft(1).Render(content)
	}

	columnWidth := a.width
	if columnWidth > ComposerColumnWidth {
		columnWidth = ComposerColumnWidth
	}

	// Left: health + workspace-relative path
	health := StatusOnline.Render("[ready]")
	if a.chatModel.GetModel() == "" {
		health = StatusConnecting.Render("[! no model]")
	}

	path := displayWorkspacePath(a.workspacePath)
	if path == "" {
		if cwd, err := os.Getwd(); err == nil {
			path = displayWorkspacePath(cwd)
		}
	}
	if path == "" {
		path = "workspace"
	}
	left := health + " " + HelpDimStyle.Render(path)

	// Right: context usage + cost + keybind hint
	var telemetry []string
	if a.contextLen > 0 {
		used := a.estTokens
		if used > a.contextLen {
			used = a.contextLen
		}
		telemetry = append(telemetry, fmt.Sprintf("ctx %s/%s (%d%% left)",
			formatTokenCount(used), formatTokenCount(a.contextLen),
			100-used*100/a.contextLen))
	}
	if a.costTotal > 0 {
		telemetry = append(telemetry, "$"+fmt.Sprintf("%.2f", a.costTotal))
	}
	telemetry = append(telemetry, "ctrl+p commands")
	right := StatusHintStyle.Render(strings.Join(telemetry, " · "))

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := columnWidth - leftW - rightW
	if gap < 2 {
		gap = 2
	}
	content := left + strings.Repeat(" ", gap) + right

	if a.width > columnWidth {
		content = lipgloss.PlaceHorizontal(a.width, lipgloss.Center, content)
	}

	return StatusBarStyle.Width(a.width).PaddingBottom(1).PaddingLeft(1).Render(content)
}

// formatTokenCount renders a token count compactly (12k, 8182, 1.2m).
func formatTokenCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func displayWorkspacePath(path string) string {
	if path == "" {
		return ""
	}

	clean := filepath.Clean(path)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, clean); err == nil {
			if rel == "." {
				return "~"
			}
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return filepath.Join("~", rel)
			}
		}
	}

	slashPath := filepath.ToSlash(clean)
	parts := strings.Split(slashPath, "/")
	if len(parts) >= 5 && parts[1] == "mnt" && strings.EqualFold(parts[3], "Users") {
		if len(parts) == 5 {
			return "~"
		}
		return "~/" + strings.Join(parts[5:], "/")
	}

	return clean
}

// ---------------------------------------------------------------------------
// View switching helpers
// ---------------------------------------------------------------------------
