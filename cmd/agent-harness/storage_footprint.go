package main

import (
	"os"
	"path/filepath"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/format"
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
		format.HumanBytes(total),
		config.DataHome(),
		sessions.files,
		format.HumanBytes(sessions.bytes),
		format.HumanBytes(audit.bytes),
		format.HumanBytes(logs.bytes),
		format.HumanBytes(toolResults.bytes),
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

