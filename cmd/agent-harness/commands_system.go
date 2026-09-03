package main

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/persona"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/commands"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"path/filepath"
)

func (app *App) initCommandsSystem() {
	app.cmdRegistry.Register("version", "Show version",
		commands.VersionHandler(Version, sprintf("Built: %s Git: %s", BuildTime, GitSHA)))

	app.cmdRegistry.Register("config", "Show or update configuration (usage: /config [provider|endpoint|model|key] <val>)",
		commands.ConfigHandler(
			func() string {
				return app.formatConfig()
			},
			func(key, value string) (string, error) {
				return app.updateConfiguration(key, value)
			},
		))

	app.cmdRegistry.Register("settings", "Open settings dashboard tab",
		commands.SettingsHandler())

	app.cmdRegistry.Register("permissions", "Show or change permission mode",
		commands.PermissionsHandler(
			func() string { return app.executionMode.String() },
			func(m string) error {
				mode, err := approval.ParseExecutionMode(m)
				if err != nil {
					return err
				}
				app.executionMode = mode
				return nil
			},
			func() string { return app.getPermissionsReport() },
		))

	app.cmdRegistry.Register("agents", "Show available agents",
		commands.AgentsHandler(func(args string) string {
			return app.formatAgentsList()
		}))

	app.cmdRegistry.Register("skills", "Show available skills",
		commands.SkillsHandler(func(args string) string {
			skillsDir := filepath.Join(app.cwd, ".workspace", "skills")
			reg, _ := skills.LoadFromDirectory(skillsDir)
			if args != "" {
				if sk, ok := reg.Get(args); ok {
					return formatSkillDetail(sk)
				}
				return sprintf("Skill not found: %s", args)
			}
			return formatSkillsList(reg.All())
		}))

	app.cmdRegistry.Register("workspace", "Show workspace information",
		commands.WorkspaceHandler(func() string {
			return app.getWorkspaceInfo()
		}))

	app.cmdRegistry.Register("logout", "Log out and clear credentials",
		commands.LogoutHandler(func() error {
			return app.logout()
		}))

	app.cmdRegistry.Register("audit", "Show recent tool activity",
		commands.AuditHandler(func() string {
			return app.getAuditLog()
		}))

	app.cmdRegistry.Register("login", "Log in with new credentials",
		commands.LoginHandler(func() error {
			return app.startLogin()
		}))

	app.cmdRegistry.Register("provider", "Switch provider and pick a model",
		commands.LoginHandler(func() error {
			return app.startProviderPicker()
		}))

	app.cmdRegistry.Register("models", "Pick a model from the full list",
		commands.LoginHandler(func() error {
			return app.startModelPicker()
		}))

	app.cmdRegistry.Register("persona", "Switch behavior mode",
		commands.PersonaHandler(
			func() string { return app.session.Persona },
			func(p string) error {
				parsed, err := persona.Parse(p)
				if err != nil {
					return err
				}
				app.session.Persona = parsed.String()
				app.sessionManager.SetCurrent(app.session)
				if _, err := app.sessionManager.SaveCurrent(); err != nil {
					diag.Error("session.save.persona", err)
				}
				return nil
			},
			formatPersonaList,
		))

	app.cmdRegistry.Register("reset", "Reset credentials and sessions",
		commands.ResetHandler(func() error {
			return app.reset()
		}))

	app.cmdRegistry.Register("quit", "Exit the application", commands.QuitHandler())
	app.cmdRegistry.Register("exit", "Exit the application", commands.QuitHandler())
}
