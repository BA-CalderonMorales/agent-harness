package main

import (
	"fmt"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"slices"
)

func (app *App) initCommandsCore() {
	app.cmdRegistry = commands.NewSlashRegistry()

	app.cmdRegistry.Register("help", "Show available commands",
		commands.HelpHandler(app.cmdRegistry))

	app.cmdRegistry.Register("status", "Show session status",
		commands.StatusHandler(func() string {
			return app.sessionManager.FormatSessionReport()
		}))

	app.cmdRegistry.Register("clear", "Clear the session history",
		commands.ClearHandler(func() error {
			app.session = app.session.Clear()
			app.sessionManager.SetCurrent(app.session)
			_, _ = app.sessionManager.SaveCurrent()
			app.refreshTelemetry(app.tuiApp)
			return nil
		}, nil))

	app.cmdRegistry.Register("compact", "Compact session to reduce token usage",
		commands.CompactHandler(func() (string, error) {
			cfg := state.DefaultCompactionConfig()
			if app.client != nil {
				cfg.Summarizer = func(msgs []types.Message) (string, error) {
					return app.summarizeMessages(msgs)
				}
			}
			result := app.session.Compact(cfg)
			app.session = result.CompactedSession
			app.sessionManager.SetCurrent(app.session)
			if _, err := app.sessionManager.SaveCurrent(); err != nil {
				return "", fmt.Errorf("save compacted session: %w", err)
			}
			app.refreshTelemetry(app.tuiApp)
			return sprintf("Compacted: removed %d messages, kept %d", result.RemovedCount, result.KeptCount), nil
		}))

	app.cmdRegistry.Register("cost", "Show token usage and cost",
		commands.CostHandler(func() string {
			if app.costTracker == nil {
				return "Cost tracking is not active."
			}
			return app.costTracker.Summary()
		}))

	app.cmdRegistry.Register("model", "Show or change the current model",
		commands.ModelHandler(
			func() string { return app.session.Model },
			func(m string) error {
				app.session.Model = m
				if app.costTracker != nil {
					app.costTracker.SetModel(m)
				}
				app.commitConfigChange()
				app.sessionManager.SetCurrent(app.session)
				_, _ = app.sessionManager.SaveCurrent()
				app.refreshTelemetry(app.tuiApp)
				return nil
			},
			func() []string {
				items := app.getModelItems()
				names := make([]string, len(items))
				for i, item := range items {
					names[i] = item.ID
				}
				return names
			},
		))

	app.cmdRegistry.Register("current-model", "Show the current model",
		commands.CurrentModelHandler(func() string { return app.session.Model }))

	app.cmdRegistry.Register("effort", "Show or cycle reasoning effort (usage: /effort [low|medium|high])",
		commands.ModelHandler(
			func() string { return app.config.Effort },
			func(e string) error {
				if e == "" {
					e = nextEffort(app.config.Effort)
				} else if !slices.Contains(config.EffortLevels, e) {
					return fmt.Errorf("invalid effort '%s'. Available: low, medium, high", e)
				}
				app.config.Effort = e
				app.commitConfigChange()
				app.rebuildLLMClient()
				if app.tuiApp != nil {
					app.tuiApp.Send(tui.StatusMsg{Text: sprintf("Reasoning effort: %s", e), Type: "success"})
				}
				return nil
			},
			func() []string { return config.EffortLevels },
		))

	app.cmdRegistry.Register("export", "Export conversation to file",
		commands.ExportHandler(func(args string) (string, error) {
			return exportSession(app.session, args, app.config.APIKey)
		}))

	app.cmdRegistry.Register("session", "Manage sessions",
		commands.SessionHandler(
			func() string {
				sessions, err := app.sessionManager.ListSessions()
				if err != nil {
					return sprintf("Failed to list sessions: %v", err)
				}
				return formatSessionList(sessions, app.session.ID)
			},
			func(id string) error {
				loaded, err := app.sessionManager.LoadSession(id)
				if err != nil {
					return err
				}
				app.session = loaded
				app.sessionManager.SetCurrent(loaded)
				return nil
			},
		))

	app.cmdRegistry.Register("plan", "Toggle plan mode (outline before executing)",
		commands.PlanHandler(
			func() bool { return app.planMode },
			func(enabled bool) string {
				app.planMode = enabled
				if enabled {
					return "Plan mode enabled. Agent will outline steps before executing."
				}
				return "Plan mode disabled. Agent will execute directly."
			},
		))

	app.cmdRegistry.Register("memory", "Show system prompt and context state",
		commands.MemoryHandler(func() string {
			return app.getMemoryInfo()
		}))

	app.cmdRegistry.Register("limit", "Show or set the session tool-call limit",
		commands.LimitHandler(
			func() int {
				if app.session != nil {
					return app.session.ToolLimit
				}
				return 0
			},
			func(n int) error {
				app.session.ToolLimit = n
				app.session.UpdatedAt = time.Now()
				app.session.Version++
				app.sessionManager.SetCurrent(app.session)
				_, err := app.sessionManager.SaveCurrent()
				return err
			},
		))

	app.cmdRegistry.Register("init", "Initialize project with standard files",
		commands.InitHandler(func(projectType string) (string, error) {
			return app.initProject(projectType)
		}))

}
