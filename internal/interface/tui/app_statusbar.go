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
		content = lipgloss.NewStyle().MaxWidth(a.width).Render(content)
		return StatusBarStyle.Width(a.width).PaddingTop(1).PaddingBottom(1).Render(content)
	}

	// The footer spans the full terminal width, keeping flush edges with
	// the composer block above it; a spacer row separates the bar from the
	// mode line.
	columnWidth := a.width

	// Left: health + workspace-relative path. The badge reflects the
	// provider probe (checking/ready/warning/unavailable/misconfigured),
	// never a model-empty proxy - a first-run user with no key must see
	// the gap, and the (l: login) handle is the fix, not prose.
	health := StatusOnline.Render("[ready]")
	switch a.providerReadiness {
	case 0: // checking
		health = StatusConnecting.Render("[checking…]")
	case 2: // warning
		health = StatusConnecting.Render("[! warning]")
	case 3: // unavailable
		health = StatusOffline.Render("[! not connected] (l: login)")
	case 4: // misconfigured
		health = StatusOffline.Render("[! setup required] (l: login)")
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

	// Right segments in priority order (drop from the end as width shrinks:
	// hint first, then cost; context usage survives the longest).
	var telemetry []string
	mobileTmux := isMobilePane(a.width) && inTmux()
	if mobileTmux {
		if a.mouseCapture {
			telemetry = append(telemetry, `"m" copy`)
		} else {
			telemetry = append(telemetry, `"m" gestures`)
		}
		telemetry = append(telemetry, `"i" type`)
	}
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
	if !mobileTmux {
		telemetry = append(telemetry, `"?" help`, `"m" copy`)
	}

	// Drop right segments until the health badge, a minimum path, the gap,
	// and the remaining segments all fit end-to-end inside the column. The
	// path cap is included in the decision so the bar can never overflow.
	const (
		gapMin       = 2
		minPathWidth = 8
		maxPathWidth = 56
	)
	healthW := lipgloss.Width(health)
	right := ""
	rightW := 0
	pathMax := minPathWidth
	for len(telemetry) > 0 {
		candidate := StatusHintStyle.Render(strings.Join(telemetry, " · "))
		cw := lipgloss.Width(candidate)
		budget := columnWidth - gapMin - healthW - 1 - cw
		effective := budget
		if effective > maxPathWidth {
			effective = maxPathWidth
		}
		if effective >= minPathWidth {
			right, rightW, pathMax = candidate, cw, effective
			break
		}
		telemetry = telemetry[:len(telemetry)-1]
	}
	path = fitPath(path, pathMax)

	left := health + " " + HelpDimStyle.Render(path)
	leftW := lipgloss.Width(left)
	gap := columnWidth - leftW - rightW
	if gap < gapMin {
		gap = gapMin
	}
	content := left + strings.Repeat(" ", gap) + right
	content = lipgloss.NewStyle().MaxWidth(a.width).Render(content)

	return StatusBarStyle.Width(a.width).PaddingTop(1).PaddingBottom(1).Render(content)
}

// fitPath renders a path within a width budget, keeping the first segment
// (e.g. ~) and the trailing segments and ellipsizing the middle:
// ~/…/working/agent-harness.
func fitPath(path string, max int) string {
	if lipgloss.Width(path) <= max {
		return path
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) == 1 {
		if lipgloss.Width(path) <= max {
			return path
		}
		return path[:max-1] + "…"
	}
	head := parts[0]

	// Longest surviving tail (up to three segments).
	tail := ""
	for i := 1; i <= 3 && len(parts)-i >= 1; i++ {
		candidate := strings.Join(parts[len(parts)-i:], "/")
		short := head + "/…/" + candidate
		if lipgloss.Width(short) <= max {
			tail = candidate
		} else {
			break
		}
	}
	if tail != "" {
		return head + "/…/" + tail
	}

	// Even the last segment alone is too long: hard-ellipsize the tail.
	last := parts[len(parts)-1]
	if lipgloss.Width(head)+3 >= max {
		return head[:max-2] + "…"
	}
	avail := max - lipgloss.Width(head) - 3
	if avail < 1 {
		avail = 1
	}
	return head + "/…/" + last[:avail-1] + "…"
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
