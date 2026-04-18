package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/agent"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/approval"
	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/llm"
	"github.com/BA-CalderonMorales/agent-harness/internal/skills"
	"github.com/BA-CalderonMorales/agent-harness/internal/ui"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// getSessionInfos returns session info for TUI.
func (app *App) getSessionInfos() []tui.SessionInfo {
	sessions, err := app.sessionManager.ListSessions()
	if err != nil {
		sessions = []state.SessionMetadata{}
	}

	// Ensure current session is included
	currentSession := app.sessionManager.GetCurrent()
	if currentSession != nil {
		sessions = ensureCurrentSession(sessions, currentSession.ID)
	}

	return convertToSessionInfos(sessions, currentSession)
}

// ensureCurrentSession adds current session to list if missing.
func ensureCurrentSession(sessions []state.SessionMetadata, currentID string) []state.SessionMetadata {
	for _, s := range sessions {
		if s.ID == currentID {
			return sessions
		}
	}
	// Current session not in list - find it and prepend
	return sessions
}

// convertToSessionInfos converts SessionMetadata to SessionInfo.
func convertToSessionInfos(sessions []state.SessionMetadata, current *state.Session) []tui.SessionInfo {
	var infos []tui.SessionInfo
	for _, s := range sessions {
		infos = append(infos, tui.SessionInfo{
			ID:           s.ID,
			Title:        sprintf("Session %s", s.ID[:8]),
			MessageCount: s.MessageCount,
			Turns:        s.Turns,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
			Model:        s.Model,
			IsActive:     current != nil && s.ID == current.ID,
		})
	}
	return infos
}

// getSettings returns current settings for TUI.
func (app *App) getSettings() []tui.Setting {
	return []tui.Setting{
		{Key: "model", Label: "Model", Value: app.session.Model, Description: "The AI model to use", Type: "string"},
		{Key: "provider", Label: "Provider", Value: app.config.Provider, Description: "API provider", Type: "string"},
		{Key: "permissions", Label: "Permission Mode", Value: app.config.PermissionMode.String(), Description: "Tool permission level", Type: "choice", Options: []string{"read-only", "workspace-write", "danger-full-access"}},
		{Key: "execution_mode", Label: "Execution Mode", Value: app.executionMode.String(), Description: "Command approval mode", Type: "choice", Options: []string{"interactive", "yolo"}},
		{Key: "perm_read", Label: "Allow Read", Value: "", Description: "Allow read/search tools", Type: "bool", BoolValue: app.config.PermRead},
		{Key: "perm_write", Label: "Allow Write", Value: "", Description: "Allow write/edit tools", Type: "bool", BoolValue: app.config.PermWrite},
		{Key: "perm_delete", Label: "Allow Delete", Value: "", Description: "Allow delete/remove tools", Type: "bool", BoolValue: app.config.PermDelete},
		{Key: "perm_execute", Label: "Allow Execute", Value: "", Description: "Allow bash/execute tools", Type: "bool", BoolValue: app.config.PermExecute},
	}
}

// getModelItems returns available models for TUI.
func (app *App) getModelItems() []tui.ModelItem {
	provider := app.config.Provider
	if provider == "" {
		provider = "openrouter"
	}

	return getModelsForProvider(provider, app.session.Model)
}

// getModelsForProvider returns models appropriate for the provider.
func getModelsForProvider(provider, currentModel string) []tui.ModelItem {
	switch provider {
	case "openai":
		return []tui.ModelItem{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4o"},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4o-mini"},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", ContextLen: 128000, IsDefault: currentModel == "gpt-4-turbo"},
		}
	case "anthropic":
		return []tui.ModelItem{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: "anthropic", ContextLen: 200000, IsDefault: currentModel == "claude-3-5-sonnet-20241022"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", ContextLen: 200000, IsDefault: currentModel == "claude-3-opus-20240229"},
		}
	case "ollama":
		return []tui.ModelItem{
			{ID: "gemma4:2b", Name: "Gemma 4 E2B (Fast)", Provider: "ollama", ContextLen: 128000, IsDefault: currentModel == "gemma4:2b"},
			{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Provider: "ollama", ContextLen: 128000, IsDefault: currentModel == "llama3.2:3b"},
		}
	default:
		return []tui.ModelItem{
			{ID: "nvidia/nemotron-3-super-120b-a12b:free", Name: "Nemotron 3 Super 120B (free)", Provider: "openrouter", ContextLen: 128000, IsDefault: currentModel == "nvidia/nemotron-3-super-120b-a12b:free"},
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", ContextLen: 200000, IsDefault: currentModel == "anthropic/claude-3.5-sonnet"},
			{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", ContextLen: 128000, IsDefault: currentModel == "openai/gpt-4o"},
		}
	}
}

// getWorkspaceInfo returns formatted workspace information.
func (app *App) getWorkspaceInfo() string {
	var b strings.Builder

	b.WriteString(sprintf("Current directory: %s\n", app.cwd))

	if app.gitContext != nil && app.gitContext.IsRepo {
		b.WriteString(sprintf("Git repository: %s\n", app.gitContext.Root))
		if app.gitContext.Branch != "" {
			b.WriteString(sprintf("  Branch: %s\n", app.gitContext.Branch))
		}
	} else {
		b.WriteString("Git: not a repository\n")
	}

	if app.session != nil {
		b.WriteString(sprintf("\nActive session: %s\n", app.session.ID[:8]))
		b.WriteString(sprintf("  Model: %s\n", app.session.Model))
		b.WriteString(sprintf("  Messages: %d\n", len(app.session.Messages)))
		b.WriteString(sprintf("  Turns: %d\n", app.session.Turns))
	}

	b.WriteString(sprintf("\nPermission mode: %s\n", app.config.PermissionMode.String()))
	b.WriteString(sprintf("Provider: %s\n", app.config.Provider))

	return b.String()
}

// buildSystemPrompt constructs the system prompt.
func (app *App) buildSystemPrompt() string {
	gitContext := ""
	if app.gitContext != nil && app.gitContext.IsRepo {
		gitContext = sprintf("%s", app.gitContext.Root)
		if app.gitContext.Branch != "" {
			gitContext += sprintf(" (branch: %s, commit: %s)", app.gitContext.Branch, app.gitContext.Commit)
		}
		if app.gitContext.HasChanges {
			gitContext += " — has uncommitted changes"
		}
	}

	skillPrompts := loadSkillPrompts()

	var recentCommits, statusFiles, topFiles []string
	if app.gitContext != nil && app.gitContext.IsRepo {
		recentCommits = app.gitContext.RecentCommits
		statusFiles = app.gitContext.StatusFiles
		topFiles = app.gitContext.TopLevelFiles
	}

	cfg := agent.SystemPromptConfig{
		PersonaName:      "Agent",
		GitContext:       gitContext,
		PermissionMode:   app.config.PermissionMode.String(),
		WorkingDirectory: app.cwd,
		Skills:           skillPrompts,
		RecentCommits:    recentCommits,
		StatusFiles:      statusFiles,
		TopLevelFiles:    topFiles,
		PlanMode:         app.session.PlanMode,
	}

	return agent.BuildSystemPrompt(cfg)
}

// loadSkillPrompts loads skill prompts from directory.
func loadSkillPrompts() []string {
	skillReg, err := skills.LoadFromDirectory(".agent-harness/skills")
	if err != nil {
		return nil
	}

	var prompts []string
	for _, sk := range skillReg.All() {
		prompts = append(prompts, sk.FormatPrompt())
	}
	return prompts
}

// buildWelcomeMessage creates a contextual welcome message.
func (app *App) buildWelcomeMessage() string {
	var parts []string
	parts = append(parts, sprintf("Agent Harness %s", Version))

	if app.session != nil && len(app.session.Messages) > 0 {
		parts = append(parts, sprintf("  Resumed session %s (%d messages, %d turns)",
			app.session.ID[:8], len(app.session.Messages), app.session.Turns))
	}

	if app.gitContext != nil && app.gitContext.IsRepo {
		parts = append(parts, sprintf("  Git: %s (%s)", app.gitContext.Root, app.gitContext.Branch))
		if len(app.gitContext.RecentCommits) > 0 {
			parts = append(parts, sprintf("  Last commit: %s", app.gitContext.RecentCommits[0]))
		}
		if app.gitContext.HasChanges {
			parts = append(parts, "  Status: uncommitted changes present")
		} else {
			parts = append(parts, "  Status: clean")
		}
	} else {
		parts = append(parts, sprintf("  Dir: %s", app.cwd))
	}

	if projType := detectProjectType(app.cwd); projType != "" {
		parts = append(parts, sprintf("  Project: %s", projType))
	}

	if app.session.PlanMode {
		parts = append(parts, "  Mode: plan — outline before executing")
	}

	parts = append(parts, "")
	parts = append(parts, "Type /help for commands")
	return strings.Join(parts, "\n")
}

// detectProjectType guesses the project language from common marker files.
func detectProjectType(dir string) string {
	markers := []struct {
		file string
		name string
	}{
		{"go.mod", "Go"},
		{"package.json", "Node"},
		{"Cargo.toml", "Rust"},
		{"pyproject.toml", "Python"},
		{"requirements.txt", "Python"},
		{"composer.json", "PHP"},
		{"Gemfile", "Ruby"},
		{"pom.xml", "Java"},
		{"build.gradle", "Java"},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			return m.name
		}
	}
	return ""
}

// summarizeMessages sends messages to the LLM for summarization.
func (app *App) summarizeMessages(msgs []types.Message) (string, error) {
	if app.client == nil {
		return "", fmt.Errorf("no LLM client")
	}
	var b strings.Builder
	b.WriteString("Summarize the following conversation concisely. Preserve key decisions, facts, and context:\n\n")
	for _, msg := range msgs {
		b.WriteString(fmt.Sprintf("%s: ", msg.Role))
		for _, block := range msg.Content {
			switch blk := block.(type) {
			case types.TextBlock:
				b.WriteString(blk.Text)
			case types.ToolUseBlock:
				b.WriteString(fmt.Sprintf("[tool: %s]", blk.Name))
			case types.ToolResultBlock:
				b.WriteString(fmt.Sprintf("[result: %v]", blk.Content))
			}
		}
		b.WriteString("\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llm.Request{
		Messages: []types.Message{
			{UUID: generateUUID(), Role: types.RoleUser, Content: []types.ContentBlock{types.TextBlock{Text: b.String()}}, Timestamp: time.Now()},
		},
		SystemPrompt: "You are a context summarizer. Summarize conversation history in 2-3 sentences. Be concise but preserve all key facts, decisions, and context.",
		Model:        app.session.Model,
		MaxTokens:    512,
	}

	stream, err := app.client.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for event := range stream {
		switch e := event.(type) {
		case types.LLMTextDelta:
			result.WriteString(e.Delta)
		case types.LLMError:
			return result.String(), e.Error
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// initProject scaffolds standard files for a new project.
func (app *App) initProject(projectType string) (string, error) {
	files := map[string]string{
		"README.md":  fmt.Sprintf("# %s\n\nProject initialized with agent-harness.\n", filepath.Base(app.cwd)),
		".gitignore": "# Agent harness\n.agent-harness/sessions/\nbuild/\ndist/\n\n# OS\n.DS_Store\nThumbs.db\n",
		"LICENSE":    "MIT License\n\nCopyright (c) " + fmt.Sprintf("%d", time.Now().Year()) + "\n\nPermission is hereby granted...\n",
	}

	switch projectType {
	case "go", "Go":
		files["go.mod"] = fmt.Sprintf("module %s\n\ngo 1.24\n", filepath.Base(app.cwd))
		files["main.go"] = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, world!\")\n}\n"
		files["Makefile"] = ".PHONY: build run test\n\nbuild:\n\tgo build -o build/app .\n\nrun:\n\tgo run .\n\ntest:\n\tgo test ./...\n"
	case "node", "Node":
		files["package.json"] = fmt.Sprintf("{\n  \"name\": \"%s\",\n  \"version\": \"0.1.0\",\n  \"main\": \"index.js\"\n}\n", filepath.Base(app.cwd))
		files["index.js"] = "console.log('Hello, world!');\n"
	}

	created := []string{}
	for name, content := range files {
		path := filepath.Join(app.cwd, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("failed to write %s: %w", name, err)
			}
			created = append(created, name)
		}
	}

	if len(created) == 0 {
		return "No files created — they already exist.", nil
	}
	return fmt.Sprintf("Initialized %s project. Created: %s", projectType, strings.Join(created, ", ")), nil
}

// isReadOnlyTool checks if a tool is read-only.
func isReadOnlyTool(name string) bool {
	readOnly := []string{"read", "glob", "grep", "search", "web_fetch", "web_search"}
	return stringSliceContains(readOnly, name)
}

// isDangerousTool checks if a tool is potentially dangerous.
func isDangerousTool(name string) bool {
	dangerous := []string{"bash", "write", "edit"}
	return stringSliceContains(dangerous, name)
}

// stringSliceContains checks if string slice contains value.
func stringSliceContains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// generateUUID generates a simple timestamp-based UUID.
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// extractCommandForDisplay extracts command string from tool input for display.
func (app *App) extractCommandForDisplay(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "bash", "shell":
		if cmd, ok := toolInput["command"].(string); ok {
			return cmd
		}
	case "write", "edit":
		return extractWriteEditDisplay(toolInput)
	default:
		return extractGenericDisplay(toolInput, toolName)
	}
	return sprintf("[%s]", toolName)
}

// extractWriteEditDisplay extracts display for write/edit tools.
func extractWriteEditDisplay(toolInput map[string]any) string {
	var parts []string
	if path, ok := toolInput["path"].(string); ok {
		parts = append(parts, path)
	}
	if content, ok := toolInput["content"].(string); ok {
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			display := lines[0]
			if len(display) > 50 {
				display = display[:47] + "..."
			}
			parts = append(parts, display)
		}
	}
	return strings.Join(parts, " - ")
}

// extractGenericDisplay extracts display for generic tools.
func extractGenericDisplay(toolInput map[string]any, toolName string) string {
	var parts []string
	for k, v := range toolInput {
		if k != "command" && k != "content" {
			parts = append(parts, sprintf("%s=%v", k, v))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return sprintf("[%s]", toolName)
}

// requestCommandApproval requests user approval for a command.
func (app *App) requestCommandApproval(toolName, command string, toolInput map[string]any) (approval.Decision, error) {
	if app.tuiApp == nil {
		return approval.DecisionReject, fmt.Errorf("TUI not available")
	}

	cmdID := generateUUID()
	isDestructive := checkDestructive(toolName, command)

	req := approval.NewApprovalRequest(approval.CommandInfo{
		ID:            cmdID,
		ToolName:      toolName,
		DisplayName:   toolName,
		Command:       command,
		Description:   approval.FormatCommandForDisplay(toolName, command),
		IsDestructive: isDestructive,
		Timestamp:     time.Now(),
	})

	app.tuiApp.Send(tui.ApprovalRequestMsg{Request: req})

	select {
	case decision := <-req.Response:
		return decision, nil
	case <-req.Context.Done():
		return approval.DecisionReject, req.Context.Err()
	}
}

// checkDestructive determines if a command is destructive.
func checkDestructive(toolName, command string) bool {
	if toolName == "bash" || toolName == "shell" {
		if strings.Contains(command, "rm ") || strings.Contains(command, "dd ") {
			return true
		}
	}
	return toolName == "write" || toolName == "edit"
}

// interactiveSetup guides user through initial configuration.
func (app *App) interactiveSetup(credManager *config.CredentialManager) error {
	fmt.Println()
	fmt.Println(ui.HeaderStyle.Render("  Welcome to Agent Harness"))
	fmt.Println()
	fmt.Println("  Let's get you set up.")
	fmt.Println()

	provider := promptProvider()
	app.config.Provider = provider

	if provider == "ollama" {
		app.config.APIKey = "ollama"
		fmt.Println("  Ollama uses local models - no API key required")
	} else {
		apiKey := promptAPIKey(provider)
		if apiKey == "" {
			return errf("API key cannot be empty")
		}
		app.config.APIKey = apiKey
		fmt.Println("  " + ui.RenderSuccess("API key received"))
	}

	model := promptModel(provider)
	app.config.Model = model

	fmt.Println()
	fmt.Println("  Credentials will be encrypted.")
	fmt.Println()

	secureCfg := &config.SecureConfig{
		Provider: app.config.Provider,
		APIKey:   app.config.APIKey,
		Model:    app.config.Model,
	}

	if err := credManager.SaveSecure(secureCfg); err != nil {
		return errf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + ui.RenderSuccess("Credentials saved"))
	fmt.Println()

	return nil
}

// promptProvider prompts user for API provider.
func promptProvider() string {
	fmt.Println("  Choose an API provider:")
	fmt.Println("    1) OpenRouter")
	fmt.Println("    2) OpenAI")
	fmt.Println("    3) Anthropic")
	fmt.Println("    4) Ollama (Local)")
	fmt.Print("  Enter choice (1-4) [1]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "2":
		return "openai"
	case "3":
		return "anthropic"
	case "4":
		return "ollama"
	default:
		return "openrouter"
	}
}

// promptAPIKey prompts user for API key.
func promptAPIKey(provider string) string {
	fmt.Printf("  Enter your %s API key: ", provider)
	apiKey, err := config.PromptPassword("")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(apiKey)
}

// promptModel prompts user for model selection.
func promptModel(provider string) string {
	defaultModel := getDefaultModel(provider)

	fmt.Printf("  Model [%s]: ", defaultModel)
	reader := bufio.NewReader(os.Stdin)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)

	model = resolveModelInput(model, provider)

	if model != "" {
		return model
	}
	return defaultModel
}

// getDefaultModel returns default model for provider.
func getDefaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	case "ollama":
		return "gemma4:2b"
	default:
		return "nvidia/nemotron-3-super-120b-a12b:free"
	}
}

// resolveModelInput resolves numeric or empty model input.
func resolveModelInput(input, provider string) string {
	switch input {
	case "1":
		if provider == "openai" {
			return "gpt-4o"
		} else if provider == "anthropic" {
			return "claude-3-5-sonnet-20241022"
		}
		return "nvidia/nemotron-3-super-120b-a12b:free"
	case "2":
		if provider == "openai" {
			return "gpt-4o-mini"
		} else if provider == "anthropic" {
			return "claude-3-opus-20240229"
		}
		return "anthropic/claude-3.5-sonnet"
	case "3":
		if provider == "openai" {
			return "gpt-4-turbo"
		}
		return "openai/gpt-4o"
	default:
		return input
	}
}
