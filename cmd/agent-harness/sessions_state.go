package main

import (
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"sort"
	"time"
)

// getSessionInfos returns session info for TUI.
func (app *App) getSessionInfos() []tui.SessionInfo {
	sessions, err := app.sessionManager.ListSessions()
	if err != nil {
		sessions = []state.SessionMetadata{}
	}

	// Ensure current session is included (a fresh, not-yet-saved session
	// is invisible on disk and would otherwise vanish from the list the
	// moment it is created).
	currentSession := app.sessionManager.GetCurrent()
	sessions = ensureCurrentSession(sessions, currentSession)

	return convertToSessionInfos(sessions, currentSession)
}

// ensureCurrentSession adds the current session to the list when it is
// missing (fresh in-memory session not yet persisted). Dedupes
// defensively: the disk list may already contain it.
func ensureCurrentSession(sessions []state.SessionMetadata, current *state.Session) []state.SessionMetadata {
	if current == nil || current.ID == "" {
		return sessions
	}
	for _, s := range sessions {
		if s.ID == current.ID {
			return sessions
		}
	}
	return append(sessions, current.GetMetadata())
}

// convertToSessionInfos converts SessionMetadata to SessionInfo.
func convertToSessionInfos(sessions []state.SessionMetadata, current *state.Session) []tui.SessionInfo {
	infos := make([]tui.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		infos = append(infos, tui.SessionInfo{
			ID:           s.ID,
			Title:        sprintf("Session %s", s.ID[:8]),
			MessageCount: s.MessageCount,
			Turns:        s.Turns,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
			Model:        s.Model,
			IsActive:     current != nil && s.ID == current.ID,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})
	return infos
}

// getSettings returns current settings for TUI grouped by logical section.

// isReadOnlyTool checks if a tool is read-only.
func isReadOnlyTool(name string) bool {
	readOnly := []string{"read", "glob", "grep", "search", "web_fetch", "web_search"}
	return stringSliceContains(readOnly, name)
}

// isDangerousTool checks if a tool is potentially dangerous.
func isDangerousTool(name string) bool {
	dangerous := []string{"bash", "write", "edit"}
	return stringSliceContains(dangerous, name)
}

// stringSliceContains checks if string slice contains value.
func stringSliceContains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// generateUUID generates a simple timestamp-based UUID.
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// buildCommandPreview generates a preview of what a tool will do.
