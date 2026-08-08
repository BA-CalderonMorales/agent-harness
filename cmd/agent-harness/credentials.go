package main

import (
	"bufio"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"os"
	"strings"
)

// loadCredentials handles secure credential loading and migration.
func (app *App) loadCredentials(credManager *config.CredentialManager) error {
	if config.IsLocalProvider(app.config.Provider) {
		if app.config.APIKey == "" {
			app.config.APIKey = app.config.Provider
		}
		return nil
	}

	if app.config.APIKey != "" {
		return nil
	}

	if credManager.HasSecureCredentials() {
		secureCfg, err := credManager.LoadSecure()
		if err != nil {
			return app.handleCredentialError(credManager, err)
		}
		app.applySecureConfig(secureCfg)
	}

	if app.config.APIKey == "" && credManager.HasLegacyCredentials() {
		app.migrateLegacyCredentials(credManager)
	}

	if app.config.APIKey == "" {
		if err := app.interactiveSetup(credManager); err != nil {
			return errf("setup failed: %w", err)
		}
	}

	return nil
}

// handleCredentialError handles decryption failures gracefully.
func (app *App) handleCredentialError(credManager *config.CredentialManager, err error) error {
	fmt.Fprintf(os.Stderr, "\n%s\n", ui.ErrorStyle.Render("Failed to load credentials"))
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)

	fmt.Println("Would you like to:")
	fmt.Println("  1) Try again")
	fmt.Println("  2) Reset credentials and set up again")
	fmt.Print("\nChoice [1-2] [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "2" {
		if clearErr := credManager.ClearSecureConfig(); clearErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear credentials: %v\n", clearErr)
		} else {
			fmt.Println(ui.RenderSuccess("Credentials cleared. Starting fresh..."))
		}
	} else {
		return errf("credential decryption failed: %w", err)
	}
	return nil
}

// applySecureConfig applies secure configuration values.
// Environment variables take precedence over saved credentials.
func (app *App) applySecureConfig(secureCfg *config.SecureConfig) {
	app.secureConfig = secureCfg
	if secureCfg.Provider != "" && os.Getenv("AH_PROVIDER") == "" && os.Getenv("AGENT_HARNESS_PROVIDER") == "" {
		app.config.Provider = secureCfg.Provider
	}
	if secureCfg.APIKey != "" && os.Getenv("AH_API_KEY") == "" && os.Getenv("AGENT_HARNESS_API_KEY") == "" {
		app.config.APIKey = secureCfg.APIKey
	}
	if secureCfg.Model != "" && os.Getenv("AH_MODEL") == "" && os.Getenv("AGENT_HARNESS_MODEL") == "" {
		app.config.Model = secureCfg.Model
	}
}

// migrateLegacyCredentials migrates from legacy format.
func (app *App) migrateLegacyCredentials(credManager *config.CredentialManager) {
	fmt.Println("Found existing credentials in legacy format.")
	secureCfg, err := credManager.MigrateFromLegacy()
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
	} else {
		app.applySecureConfig(secureCfg)
	}
}

// initSession initializes the session manager and creates or resumes a session.
