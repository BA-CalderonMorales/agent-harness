package commands

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestAllSlashCommandsQuickCheckProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MaxSize = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: HelpHandler returns help or specific command descriptions
	properties.Property("HelpHandler: returns help or command description", prop.ForAll(
		func(arg string) bool {
			reg := NewSlashRegistry()
			reg.Register("status", "Show status", func(_ string) (string, error) { return "ok", nil })
			handler := HelpHandler(reg)

			res, err := handler(arg)
			if err != nil {
				return false
			}
			if arg == "" {
				return strings.Contains(res, "Available commands:") && strings.Contains(res, "/status")
			}
			if arg == "status" {
				return res == "/status - Show status"
			}
			return strings.Contains(res, "Unknown command:")
		},
		gen.AlphaString(),
	))

	// Property 2: ModelHandler state transitions and cycling
	properties.Property("ModelHandler: model get/set and cycle invariants", prop.ForAll(
		func(newModel string) bool {
			currentModel := "gpt-4o"
			getModel := func() string { return currentModel }
			setModel := func(m string) error {
				if m == "invalid" {
					return errors.New("invalid model")
				}
				currentModel = m
				return nil
			}
			listModels := func() []string { return []string{"gpt-4o", "claude-3-5-sonnet", "gemma4:2b"} }

			handler := ModelHandler(getModel, setModel, listModels)

			// Case 1: bare /model cycles to the next model in the list
			cycleRes, err := handler("")
			if err != nil || currentModel != "claude-3-5-sonnet" || !strings.Contains(cycleRes, "Model cycled") {
				return false
			}

			// Case 2: valid new model updates state
			if newModel != "" && newModel != "invalid" {
				setRes, err := handler(newModel)
				if err != nil || currentModel != newModel || !strings.Contains(setRes, "Model updated") {
					return false
				}
			}

			// Case 3: error propagation
			_, errErr := handler("invalid")
			return errErr != nil
		},
		gen.AlphaString(),
	))

	// Property 3: PermissionsHandler mode transitions
	properties.Property("PermissionsHandler: mode set/get and report invariants", prop.ForAll(
		func(newMode string) bool {
			currentMode := "interactive"
			getMode := func() string { return currentMode }
			setMode := func(m string) error {
				if m == "error" {
					return errors.New("invalid mode")
				}
				currentMode = m
				return nil
			}
			getReport := func() string { return "Mode: " + currentMode }

			handler := PermissionsHandler(getMode, setMode, getReport)

			if newMode == "" {
				res, err := handler("")
				return err == nil && res == "Mode: interactive"
			}

			if newMode == "error" {
				_, err := handler("error")
				return err != nil
			}

			res, err := handler(newMode)
			return err == nil && currentMode == newMode && strings.Contains(res, "Permissions updated")
		},
		gen.AlphaString(),
	))

	// Property 4: PlanHandler toggle and explicit state invariants
	properties.Property("PlanHandler: toggle on/off invariants", prop.ForAll(
		func(arg string) bool {
			planMode := false
			getMode := func() bool { return planMode }
			setMode := func(b bool) string {
				planMode = b
				if b {
					return "Plan mode enabled"
				}
				return "Plan mode disabled"
			}

			handler := PlanHandler(getMode, setMode)

			switch arg {
			case "":
				// Toggle false -> true
				res1, _ := handler("")
				if !planMode || !strings.Contains(res1, "enabled") {
					return false
				}
				// Toggle true -> false
				res2, _ := handler("")
				return !planMode && strings.Contains(res2, "disabled")
			case "on":
				res, _ := handler("on")
				return planMode && strings.Contains(res, "enabled")
			case "off":
				res, _ := handler("off")
				return !planMode && strings.Contains(res, "disabled")
			default:
				res, _ := handler(arg)
				return strings.Contains(res, "Usage: /plan")
			}
		},
		gen.OneConstOf("", "on", "off", "invalid-arg"),
	))

	// Property 5: PRHandler subcommand parsing and argument isolation
	properties.Property("PRHandler: create and list dispatch", prop.ForAll(
		func(title, body string) bool {
			createdTitle, createdBody := "", ""
			listCalled := false

			createFn := func(t, b string) (string, error) {
				createdTitle, createdBody = t, b
				return "PR created", nil
			}
			listFn := func() (string, error) {
				listCalled = true
				return "PR list", nil
			}

			handler := PRHandler(createFn, listFn)

			// Test list
			resList, err := handler("")
			if err != nil || resList != "PR list" || !listCalled {
				return false
			}

			// Test create with quotes
			if title != "" && !strings.Contains(title, "\"") && !strings.Contains(title, "\n") {
				input := fmt.Sprintf("create \"%s\" %s", title, body)
				resCreate, err := handler(input)
				if err != nil || resCreate != "PR created" {
					return false
				}
				if createdTitle != title || createdBody != strings.TrimSpace(body) {
					return false
				}
			}

			return true
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property 6: BranchHandler subcommand routing
	properties.Property("BranchHandler: list/create/switch/delete routing", prop.ForAll(
		func(branchName string) bool {
			if branchName == "" {
				return true
			}
			lastOp, lastArg := "", ""

			listFn := func() (string, error) { lastOp = "list"; return "main", nil }
			createFn := func(n string) (string, error) { lastOp = "create"; lastArg = n; return "created", nil }
			switchFn := func(n string) (string, error) { lastOp = "switch"; lastArg = n; return "switched", nil }
			deleteFn := func(n string) (string, error) { lastOp = "delete"; lastArg = n; return "deleted", nil }

			handler := BranchHandler(listFn, createFn, switchFn, deleteFn)

			// List
			handler("")
			if lastOp != "list" {
				return false
			}

			// Create
			handler("create " + branchName)
			if lastOp != "create" || lastArg != branchName {
				return false
			}

			// Switch
			handler("switch " + branchName)
			if lastOp != "switch" || lastArg != branchName {
				return false
			}

			// Delete
			handler("delete " + branchName)
			if lastOp != "delete" || lastArg != branchName {
				return false
			}

			// Unknown
			res, _ := handler("unknown-cmd")
			return strings.Contains(res, "Unknown branch command")
		},
		gen.AlphaString(),
	))

	// Property 7: ResetHandler safety guard
	properties.Property("ResetHandler: requires --confirm or -y to reset", prop.ForAll(
		func(flag string) bool {
			resetDone := false
			resetFn := func() error {
				resetDone = true
				return nil
			}

			handler := ResetHandler(resetFn)

			res, err := handler(flag)
			if err != nil {
				return false
			}

			if flag == "--confirm" || flag == "-y" {
				return resetDone && IsReset(res)
			}
			return !resetDone && strings.Contains(res, "WARNING")
		},
		gen.OneConstOf("", "no", "--confirm", "-y", "sure"),
	))

	// Property 8: PersonaHandler state transition and listing
	properties.Property("PersonaHandler: list and switch persona", prop.ForAll(
		func(personaName string) bool {
			currentPersona := "developer"
			getPersona := func() string { return currentPersona }
			setPersona := func(p string) error {
				if p == "invalid" {
					return errors.New("unknown persona")
				}
				currentPersona = p
				return nil
			}
			listPersonas := func() string { return "Available personas: developer, designer, pm, scientist, explorer" }

			handler := PersonaHandler(getPersona, setPersona, listPersonas)

			// Bare /persona cycles: developer -> the next catalog
			// persona (the mock list is a stub; cycling walks
			// persona.All(), so "designer" follows "developer").
			cycleRes, err := handler("")
			if err != nil || currentPersona != "designer" || !strings.Contains(cycleRes, "Persona updated") {
				return false
			}

			// "list" remains the catalog
			listRes, err := handler("list")
			if err != nil || listRes != listPersonas() {
				return false
			}

			// Valid switch
			if personaName != "" && personaName != "invalid" {
				res, err := handler(personaName)
				if err != nil || currentPersona != personaName || !strings.Contains(res, "Persona updated") {
					return false
				}
			}

			// Invalid switch
			_, err = handler("invalid")
			return err != nil
		},
		gen.AlphaString(),
	))

	// Property 9: SteerHandler queuing
	properties.Property("SteerHandler: message queuing", prop.ForAll(
		func(msg string) bool {
			queued := ""
			queueFn := func(m string) { queued = m }
			handler := SteerHandler(queueFn)

			if msg == "" {
				res, _ := handler("")
				return strings.Contains(res, "Usage: /steer") && queued == ""
			}

			res, err := handler(msg)
			return err == nil && res == "" && queued == msg
		},
		gen.AlphaString(),
	))

	// Property 10: Complete SlashRegistry full dispatch QuickCheck suite
	properties.Property("SlashRegistry: full registry handles all registered slash commands without error or panic", prop.ForAll(
		func(cmdName string, cmdArgs string) bool {
			if !isValidCommandName(cmdName) {
				return true
			}

			registry := NewSlashRegistry()

			// Register full suite of real slash handlers
			registry.Register("help", "Show help", HelpHandler(registry))
			registry.Register("status", "Show status", StatusHandler(func() string { return "ok" }))
			registry.Register("clear", "Clear history", ClearHandler(func() error { return nil }, nil))
			registry.Register("compact", "Compact history", CompactHandler(func() (string, error) { return "compacted", nil }))
			registry.Register("cost", "Show cost", CostHandler(func() string { return "cost: $0.00" }))
			registry.Register("current-model", "Current model", CurrentModelHandler(func() string { return "gpt-4o" }))
			registry.Register("model", "Change model", ModelHandler(func() string { return "gpt-4o" }, func(_ string) error { return nil }, func() []string { return []string{"gpt-4o"} }))
			registry.Register("export", "Export session", ExportHandler(func(p string) (string, error) { return p, nil }))
			registry.Register("session", "Manage sessions", SessionHandler(func() string { return "sessions" }, func(_ string) error { return nil }))
			registry.Register("plan", "Toggle plan mode", PlanHandler(func() bool { return false }, func(_ bool) string { return "plan" }))
			registry.Register("memory", "Show memory", MemoryHandler(func() string { return "memory" }))
			registry.Register("version", "Show version", VersionHandler("1.0.0", "build"))
			registry.Register("config", "Show config", ConfigHandler(
				func() string { return "config" },
				func(key, value string) (string, error) { return "config updated", nil },
			))
			registry.Register("permissions", "Show permissions", PermissionsHandler(func() string { return "interactive" }, func(_ string) error { return nil }, func() string { return "perm" }))
			registry.Register("agents", "Show agents", AgentsHandler(func(_ string) string { return "agents" }))
			registry.Register("skills", "Show skills", SkillsHandler(func(_ string) string { return "skills" }))
			registry.Register("quit", "Quit app", QuitHandler())

			input := "/" + cmdName
			if cmdArgs != "" {
				input += " " + cmdArgs
			}

			result, handled, err := registry.Handle(input)

			// Slash commands are always handled by SlashRegistry
			if !handled || err != nil {
				return false
			}

			// Handled result must be non-empty string
			return result != ""
		},
		gen.OneConstOf("help", "status", "clear", "compact", "cost", "current-model", "model", "export", "session", "plan", "memory", "version", "config", "permissions", "agents", "skills", "quit"),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func isValidCommandName(s string) bool {
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}
