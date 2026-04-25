package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"golang.org/x/term"
)

// runDiagnose prints a configuration diagnostic report.
// It checks all potential config sources without triggering interactive setup.
func runDiagnose() error {
	fmt.Println(ui.InfoStyle.Render("Agent Harness Configuration Diagnostic"))
	fmt.Println()

	cwd, _ := os.Getwd()
	loader := config.NewLayeredLoader(cwd)

	// 1. Environment variables
	fmt.Println(ui.InfoStyle.Render("Environment Variables"))
	envVars := []struct {
		name  string
		label string
	}{
		{"AH_API_KEY", "AH_API_KEY"},
		{"AGENT_HARNESS_API_KEY", "AGENT_HARNESS_API_KEY"},
		{"AH_PROVIDER", "AH_PROVIDER"},
		{"AGENT_HARNESS_PROVIDER", "AGENT_HARNESS_PROVIDER"},
		{"AH_MODEL", "AH_MODEL"},
		{"AGENT_HARNESS_MODEL", "AGENT_HARNESS_MODEL"},
		{"OPENROUTER_API_KEY", "OPENROUTER_API_KEY (legacy)"},
		{"OPENAI_API_KEY", "OPENAI_API_KEY (legacy)"},
		{"ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY (legacy)"},
		{"OLLAMA_HOST", "OLLAMA_HOST"},
	}

	anyEnvSet := false
	for _, ev := range envVars {
		val := os.Getenv(ev.name)
		if val != "" {
			anyEnvSet = true
			masked := maskKey(val)
			fmt.Printf("  %s %s = %s\n", ui.SuccessStyle.Render("✓"), ev.label, masked)
		}
	}
	if !anyEnvSet {
		fmt.Printf("  %s No configuration env vars set\n", ui.WarningStyle.Render("!"))
	}
	fmt.Println()

	// 2. Config files
	fmt.Println(ui.InfoStyle.Render("Config Files"))
	entries := loader.Discover()
	anyFileFound := false
	for _, entry := range entries {
		info := "missing"
		if _, err := os.Stat(entry.Path); err == nil {
			anyFileFound = true
			info = "exists"
			if data, err := os.ReadFile(entry.Path); err == nil && len(data) > 0 {
				// Show preview of file content
				preview := strings.TrimSpace(string(data))
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				lines := strings.Split(preview, "\n")
				for i, line := range lines {
					if i > 5 {
						fmt.Printf("      ... (%d more lines)\n", len(lines)-6)
						break
					}
					// Mask api_key values in preview
					if strings.Contains(line, "api_key") {
						parts := strings.SplitN(line, ":", 2)
						if len(parts) == 2 {
							line = parts[0] + ": \"<redacted>\""
						}
					}
					fmt.Printf("      %s\n", line)
				}
				continue
			}
		}
		fmt.Printf("  %s [%s] %s (%s)\n", ui.DimStyle.Render("-"), entry.Source, entry.Path, info)
	}
	if !anyFileFound {
		fmt.Printf("  %s No config files found\n", ui.WarningStyle.Render("!"))
	}
	fmt.Println()

	// 3. Secure credential store
	fmt.Println(ui.InfoStyle.Render("Secure Credential Store"))
	credManager := config.NewCredentialManager()
	if credManager.HasSecureCredentials() {
		fmt.Printf("  %s Secure credentials found\n", ui.SuccessStyle.Render("✓"))
		if term.IsTerminal(int(syscall.Stdin)) {
			secureCfg, err := credManager.LoadSecure()
			if err != nil {
				fmt.Printf("  %s Failed to decrypt: %v\n", ui.ErrorStyle.Render("✗"), err)
			} else {
				fmt.Printf("  Provider: %s\n", orDefault(secureCfg.Provider, "(not set)"))
				fmt.Printf("  Model: %s\n", orDefault(secureCfg.Model, "(not set)"))
				if secureCfg.APIKey != "" {
					fmt.Printf("  API Key: %s\n", maskKey(secureCfg.APIKey))
				} else {
					fmt.Printf("  API Key: %s\n", ui.DimStyle.Render("(not set)"))
				}
			}
		} else {
			fmt.Printf("  %s Cannot decrypt in non-interactive mode (run without --diagnose to enter password)\n", ui.WarningStyle.Render("!"))
		}
	} else {
		fmt.Printf("  %s No secure credentials found\n", ui.WarningStyle.Render("!"))
	}

	if credManager.HasLegacyCredentials() {
		fmt.Printf("  %s Legacy credentials found (will auto-migrate)\n", ui.WarningStyle.Render("!"))
	}
	fmt.Println()

	// 4. Resolved config
	fmt.Println(ui.InfoStyle.Render("Resolved Configuration"))
	layeredConfig, err := loader.Load()
	if err != nil {
		fmt.Printf("  %s Failed to load config: %v\n", ui.ErrorStyle.Render("✗"), err)
	} else {
		fmt.Printf("  Provider: %s\n", orDefault(layeredConfig.Provider, ui.DimStyle.Render("(not set)")))
		fmt.Printf("  Model: %s\n", orDefault(layeredConfig.Model, ui.DimStyle.Render("(not set)")))
		if layeredConfig.APIKey != "" {
			fmt.Printf("  API Key: %s %s\n", maskKey(layeredConfig.APIKey), ui.SuccessStyle.Render("(from config files or env)"))
		} else {
			fmt.Printf("  API Key: %s %s\n", ui.DimStyle.Render("(not set)"), ui.WarningStyle.Render("- will trigger interactive setup"))
		}
		fmt.Printf("  Permission Mode: %s\n", layeredConfig.PermissionMode.String())
	}
	fmt.Println()

	// 5. Recommendations
	fmt.Println(ui.InfoStyle.Render("Recommendations"))
	if layeredConfig != nil && layeredConfig.APIKey == "" && !credManager.HasSecureCredentials() {
		fmt.Println("  1. Set AH_API_KEY or AGENT_HARNESS_API_KEY in your environment")
		fmt.Println("  2. Or create a config file at:")
		for _, entry := range entries {
			if entry.Source == config.SourceUser {
				fmt.Printf("     %s\n", entry.Path)
				break
			}
		}
		fmt.Println("  3. Or run `agent-harness` without --diagnose to use the login wizard")
	} else if layeredConfig != nil && layeredConfig.APIKey == "" && credManager.HasSecureCredentials() {
		fmt.Println("  1. Secure credentials exist but could not be loaded")
		fmt.Println("  2. Try resetting credentials: run `agent-harness`, choose login, then logout/re-login")
	} else {
		fmt.Println("  Configuration looks complete. If issues persist, check:")
		fmt.Println("  - Is the API key valid? (test with a curl request to your provider)")
		fmt.Println("  - Is the provider URL reachable?")
	}

	return nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
