package commands

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestSlashCommandsRapid_Properties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		registry := NewSlashRegistry()

		// Register core handlers
		registry.Register("help", "Show help", HelpHandler(registry))
		registry.Register("status", "Show status", StatusHandler(func() string { return "Status: OK" }))
		registry.Register("version", "Show version", VersionHandler("0.3.6", "SHA: abc"))
		registry.Register("settings", "Open settings tab", SettingsHandler())
		registry.Register("config", "Show or edit configuration", ConfigHandler(
			func() string { return "Config: OK" },
			func(key, val string) (string, error) {
				return "Config updated: " + key + "=" + val, nil
			},
		))

		// Invariant 1: Any registered command generates non-empty output and is handled = true
		cmdName := rapid.SampledFrom([]string{"help", "status", "version", "settings", "config"}).Draw(t, "cmdName")
		args := rapid.StringMatching("[a-zA-Z0-9_ -]*").Draw(t, "args")

		input := "/" + cmdName
		if args != "" {
			input += " " + args
		}

		res, handled, err := registry.Handle(input)
		if !handled {
			t.Fatalf("Expected command %q to be handled", input)
		}
		if err != nil {
			t.Fatalf("Unexpected error for command %q: %v", input, err)
		}
		if res == "" {
			t.Fatalf("Command %q returned empty result", input)
		}

		// Invariant 2: /settings produces the __SETTINGS__ token
		if cmdName == "settings" {
			if !IsSettings(res) {
				t.Fatalf("Expected /settings to return __SETTINGS__, got %q", res)
			}
		}

		// Invariant 3: Feature-flagged commands are isolated and return Coming Soon notice
		featureCmd := rapid.StringMatching("flag_[a-z]{3,8}").Draw(t, "featureCmd")
		registry.FeatureFlag(featureCmd, "Feature coming soon")

		resFlag, handledFlag, errFlag := registry.Handle("/" + featureCmd)
		if !handledFlag || errFlag != nil {
			t.Fatalf("Expected feature-flagged command /%s to be handled without error", featureCmd)
		}
		if !strings.Contains(resFlag, "coming soon") {
			t.Fatalf("Expected 'coming soon' notice for feature flag /%s, got %q", featureCmd, resFlag)
		}
	})
}
