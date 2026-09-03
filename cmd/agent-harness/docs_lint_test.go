package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

// docSlashToken matches /word tokens in the markdown docs.
var docSlashToken = regexp.MustCompile(`/([a-z][a-z0-9-]*)`)

// docProseTokens are /word tokens in docs/usage.md that are prose, paths,
// or example values, not commands. Anything else must be a registered
// slash command, so a future /vim-style phantom fails CI instead of
// lying to users.
var docProseTokens = map[string]bool{
	"ornith-1": true, "v1": true, "manage": true, "user": true,
	"sessions": true, "runtime": true, "new-feature": true, "n": true,
	"myproject": true, "my-project": true, "home": true, "gpt-4o": true,
	"claude-3": true, "agent-harness": true,
	// exit is the /quit alias: registered as a handler but excluded from
	// palette/completions by design (slash.go), so it cannot appear in
	// GetCommandInfos; the docs row documents the alias.
	"exit": true,
}

// TestDocsUsageSlashTokensMatchRegistry is the doc-lint invariant: every
// slash token in docs/usage.md is either a registered command or a known
// prose token, and every registered command is documented. The reverse
// direction catches missing slash-table rows (registry grows, docs lag).
func TestDocsUsageSlashTokensMatchRegistry(t *testing.T) {
	app := &App{config: &config.LayeredConfig{}}
	app.initCommands()

	registered := make(map[string]bool)
	for _, info := range app.cmdRegistry.GetCommandInfos() {
		registered[strings.TrimPrefix(info.Command, "/")] = true
	}

	raw, err := os.ReadFile("../../docs/usage.md")
	if err != nil {
		t.Fatalf("read docs/usage.md: %v", err)
	}

	docTokens := make(map[string]bool)
	for _, m := range docSlashToken.FindAllStringSubmatch(string(raw), -1) {
		docTokens[m[1]] = true
	}

	var phantoms []string
	for tok := range docTokens {
		if !registered[tok] && !docProseTokens[tok] {
			phantoms = append(phantoms, tok)
		}
	}
	if len(phantoms) > 0 {
		t.Fatalf("docs/usage.md documents unregistered commands: %s. Remove the claim or register the command.", strings.Join(phantoms, ", "))
	}

}
