package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
)

// reportStorageFootprint logs the on-disk footprint of the data home
// once the TUI is up. Storage the user can see is storage the user can
// manage — the numbers land in the Settings system log, not the footer.
func (app *App) reportStorageFootprint(tuiApp *tui.App) {
	if tuiApp == nil {
		return
	}

	sessions := dirFootprint(config.DataSessions())
	audit := dirFootprint(config.DataAudit())
	logs := dirFootprint(config.DataLogs())
	toolResults := dirFootprint(config.DataToolResults())

	total := sessions.bytes + audit.bytes + logs.bytes + toolResults.bytes
	tuiApp.AddMessage("system", sprintf(
		"Storage: %s under %s (sessions %d files / %s · audit %s · logs %s · tool results %s)",
		humanBytes(total),
		config.DataHome(),
		sessions.files,
		humanBytes(sessions.bytes),
		humanBytes(audit.bytes),
		humanBytes(logs.bytes),
		humanBytes(toolResults.bytes),
	))
}

type dirUsage struct {
	files int
	bytes int64
}

// dirFootprint sums a directory tree, ignoring absent paths.
func dirFootprint(dir string) dirUsage {
	usage := dirUsage{}
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // unreadable entries do not fail the report
		}
		if !info.IsDir() {
			usage.files++
			usage.bytes += info.Size()
		}
		return nil
	})
	if err != nil {
		return dirUsage{}
	}
	return usage
}

// humanBytes renders a byte count as the largest clean unit.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	value := float64(n)
	units := []string{"K", "M", "G", "T"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1fP", value)
}
