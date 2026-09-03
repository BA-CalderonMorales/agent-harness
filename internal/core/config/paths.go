package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataHome returns the agent-harness data root: sessions, audit, logs,
// and tool results live here. Precedence: AGENT_HARNESS_DATA_HOME,
// XDG_DATA_HOME, then ~/.local/share — the XDG convention OpenCode and
// other terminal tools follow, keeping the user's home clean.
func DataHome() string {
	if env := os.Getenv("AGENT_HARNESS_DATA_HOME"); env != "" {
		return env
	}
	if env := os.Getenv("XDG_DATA_HOME"); env != "" {
		return filepath.Join(env, "agent-harness")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "agent-harness")
}

// ConfigHome returns the agent-harness config root: settings.json and
// the credential store live here. Precedence mirrors DataHome with the
// config XDG dirs. The credential store already used this location, so
// the split predates the accessor — this is the single home for it.
func ConfigHome() string {
	if env := os.Getenv("AGENT_HARNESS_CONFIG_HOME"); env != "" {
		return env
	}
	if env := os.Getenv("XDG_CONFIG_HOME"); env != "" {
		return filepath.Join(env, "agent-harness")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness")
}

// legacyHome is the pre-XDG flat directory. New installs never create
// it; existing installs are migrated once at boot.
func legacyHome() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".agent-harness")
}

// MigrateLegacyHome moves data from the legacy flat ~/.agent-harness
// layout into the XDG roots (data → DataHome, settings.json →
// ConfigHome). Best effort and idempotent: entries that cannot move are
// reported and left in place, never deleted. Returns a human summary of
// what happened, suitable for a boot notice.
func MigrateLegacyHome() string {
	legacy := legacyHome()
	entries, err := os.ReadDir(legacy)
	if err != nil {
		return "" // nothing to migrate
	}

	moved, left := 0, 0
	summary := ""
	for _, entry := range entries {
		name := entry.Name()
		src := filepath.Join(legacy, name)

		dst, known := legacyDestination(name)
		if !known {
			left++
			continue // unknown entries stay; never delete what we don't own
		}
		if _, err := os.Stat(dst); err == nil {
			// Target already exists (fresh install wrote its own):
			// the new-home copy wins, the legacy copy is left in place.
			left++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			left++
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			left++
			summary += fmt.Sprintf("\n  could not move %s: %v", name, err)
			continue
		}
		moved++
		summary += fmt.Sprintf("\n  moved %s", name)
	}

	// The legacy home goes away when everything it held moved.
	if remaining, err := os.ReadDir(legacy); err == nil && len(remaining) == 0 {
		_ = os.Remove(legacy)
	}

	// Nothing moved this run: no summary. Leftover unknown entries
	// would otherwise re-report on every boot.
	if moved == 0 {
		return ""
	}
	out := fmt.Sprintf("Migrated legacy ~/.agent-harness (%d moved", moved)
	if left > 0 {
		out += fmt.Sprintf(", %d left in place", left)
	}
	return out + ")" + summary
}

// legacyDestination maps a legacy child name to its XDG-home location.
// Unknown names return false — the migration only touches what it owns.
func legacyDestination(name string) (string, bool) {
	switch name {
	case "sessions", "audit", "logs", "tool-results":
		return filepath.Join(DataHome(), name), true
	case "settings.json":
		return filepath.Join(ConfigHome(), name), true
	default:
		return "", false
	}
}
