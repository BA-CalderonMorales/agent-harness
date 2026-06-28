package main

import (
	"context"
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
		{"AH_RUNTIME", "AH_RUNTIME"},
		{"AGENT_HARNESS_RUNTIME", "AGENT_HARNESS_RUNTIME"},
		{"AH_MODEL", "AH_MODEL"},
		{"AGENT_HARNESS_MODEL", "AGENT_HARNESS_MODEL"},
		{"AH_MODEL_PATH", "AH_MODEL_PATH"},
		{"AGENT_HARNESS_MODEL_PATH", "AGENT_HARNESS_MODEL_PATH"},
		{"AH_ENDPOINT_URL", "AH_ENDPOINT_URL"},
		{"AGENT_HARNESS_ENDPOINT_URL", "AGENT_HARNESS_ENDPOINT_URL"},
		{"AH_CONTEXT_LENGTH", "AH_CONTEXT_LENGTH"},
		{"AGENT_HARNESS_CONTEXT_LENGTH", "AGENT_HARNESS_CONTEXT_LENGTH"},
		{"AH_TEMPERATURE", "AH_TEMPERATURE"},
		{"AGENT_HARNESS_TEMPERATURE", "AGENT_HARNESS_TEMPERATURE"},
		{"AH_MAX_TOKENS", "AH_MAX_TOKENS"},
		{"AGENT_HARNESS_MAX_TOKENS", "AGENT_HARNESS_MAX_TOKENS"},
		{"AH_WORKSPACE_PATH", "AH_WORKSPACE_PATH"},
		{"AGENT_HARNESS_WORKSPACE_PATH", "AGENT_HARNESS_WORKSPACE_PATH"},
		{"AH_LOCAL_SERVER_COMMAND", "AH_LOCAL_SERVER_COMMAND"},
		{"AGENT_HARNESS_LOCAL_SERVER_COMMAND", "AGENT_HARNESS_LOCAL_SERVER_COMMAND"},
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
				fmt.Printf("  %s [%s] %s (%s)\n", ui.SuccessStyle.Render("✓"), entry.Source, entry.Path, info)
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

	layeredConfig, loadErr := loader.Load()

	// 3. Secure credential store
	fmt.Println(ui.InfoStyle.Render("Secure Credential Store"))
	credManager := config.NewCredentialManager()
	if layeredConfig != nil && config.IsLocalProvider(layeredConfig.Provider) {
		fmt.Printf("  %s Skipped for local provider\n", ui.SuccessStyle.Render("✓"))
		if credManager.HasSecureCredentials() {
			fmt.Printf("  %s Stored remote credentials exist but are not needed for this run\n", ui.DimStyle.Render("-"))
		}
	} else if credManager.HasSecureCredentials() {
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
	if loadErr != nil {
		fmt.Printf("  %s Failed to load config: %v\n", ui.ErrorStyle.Render("✗"), loadErr)
	} else {
		fmt.Printf("  Provider: %s\n", orDefault(layeredConfig.Provider, ui.DimStyle.Render("(not set)")))
		fmt.Printf("  Runtime: %s\n", orDefault(layeredConfig.Runtime, ui.DimStyle.Render("(not set)")))
		fmt.Printf("  Model: %s\n", orDefault(layeredConfig.Model, ui.DimStyle.Render("(not set)")))
		fmt.Printf("  Model Path: %s\n", orDefault(layeredConfig.ModelPath, ui.DimStyle.Render("(not set)")))
		fmt.Printf("  Endpoint URL: %s\n", orDefault(layeredConfig.EndpointURL, ui.DimStyle.Render("(provider default)")))
		fmt.Printf("  Context Length: %d\n", layeredConfig.ContextLength)
		fmt.Printf("  Temperature: %.2f\n", layeredConfig.Temperature)
		fmt.Printf("  Max Tokens: %d\n", layeredConfig.MaxTokens)
		fmt.Printf("  Workspace Path: %s\n", orDefault(layeredConfig.WorkspacePath, ui.DimStyle.Render("(current directory)")))
		if config.IsLocalProvider(layeredConfig.Provider) {
			fmt.Printf("  API Key: %s %s\n", ui.DimStyle.Render("(not required)"), ui.SuccessStyle.Render("(local provider)"))
		} else if layeredConfig.APIKey != "" {
			fmt.Printf("  API Key: %s %s\n", maskKey(layeredConfig.APIKey), ui.SuccessStyle.Render("(from config files or env)"))
		} else {
			fmt.Printf("  API Key: %s %s\n", ui.DimStyle.Render("(not set)"), ui.WarningStyle.Render("- will trigger interactive setup"))
		}
		fmt.Printf("  Permission Mode: %s\n", layeredConfig.PermissionMode.String())
	}
	fmt.Println()

	// 5. Local runtime checks
	if layeredConfig != nil && config.IsLocalProvider(layeredConfig.Provider) {
		fmt.Println(ui.InfoStyle.Render("Local Runtime"))
		for _, check := range localRuntimeChecks(context.Background(), cwd, layeredConfig) {
			switch {
			case check.OK:
				fmt.Printf("  %s %s: %s\n", ui.SuccessStyle.Render("✓"), check.Name, check.Detail)
			case check.Warning:
				fmt.Printf("  %s %s: %s\n", ui.WarningStyle.Render("!"), check.Name, check.Detail)
			default:
				fmt.Printf("  %s %s: %s\n", ui.DimStyle.Render("-"), check.Name, check.Detail)
			}
		}
		fmt.Println()
	}

	// 6. Recommendations
	fmt.Println(ui.InfoStyle.Render("Recommendations"))
	if layeredConfig != nil && config.IsLocalProvider(layeredConfig.Provider) {
		fmt.Println("  1. Start the configured local OpenAI-compatible server before chatting:")
		if layeredConfig.ServerCommand != "" {
			fmt.Printf("     %s\n", layeredConfig.ServerCommand)
		} else {
			fmt.Println("     llama-server -m /path/to/model.gguf -c 8192 --port 8080")
		}
		fmt.Println("  2. Confirm the endpoint URL matches the running server.")
	} else if layeredConfig != nil && layeredConfig.APIKey == "" && !credManager.HasSecureCredentials() {
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
