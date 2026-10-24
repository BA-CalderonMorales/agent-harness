package main

import (
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
)

// The first-run checklist has one job: name what is wrong and the key
// that fixes it, without a paragraph.

func TestReadinessReportNamesTheMissingKey(t *testing.T) {
	app := &App{
		config: &config.LayeredConfig{
			Provider:    "local",
			EndpointURL: "http://127.0.0.1:1/v1",
		},
		session: state.NewSession("test-model"),
		cwd:     t.TempDir(),
	}

	report := app.formatReadinessReport()
	for _, want := range []string{"Readiness", "workspace", "provider", "api key", "endpoint", "model", "skills", "sub-agents", "mcp", "permissions", "Next:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report is missing %q:\n%s", want, report)
		}
	}
	if !strings.Contains(report, "press l to log in") {
		t.Fatalf("report does not offer the fix for a missing key:\n%s", report)
	}
}

func TestReadinessReportIsQuietWhenReady(t *testing.T) {
	app := &App{
		config: &config.LayeredConfig{
			Provider:    "local",
			EndpointURL: "http://127.0.0.1:1/v1",
			APIKey:      "local-dummy",
		},
		session: state.NewSession("test-model"),
		cwd:     t.TempDir(),
	}

	report := app.formatReadinessReport()
	if !strings.Contains(report, "api key") || !strings.Contains(report, "set") {
		t.Fatalf("report does not show the key as set:\n%s", report)
	}
	if strings.Contains(report, "press l to log in") {
		t.Fatalf("report still asks for a key that is set:\n%s", report)
	}
}
