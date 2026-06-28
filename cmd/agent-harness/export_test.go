package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

func TestParseExportArgsDefaultsToText(t *testing.T) {
	format, path, err := parseExportArgs("", "12345678-1234")
	if err != nil {
		t.Fatalf("parseExportArgs() error = %v", err)
	}
	if format != "txt" {
		t.Fatalf("format = %q, want txt", format)
	}
	if path != "session-12345678.txt" {
		t.Fatalf("path = %q, want session-12345678.txt", path)
	}
}

func TestParseExportArgsInfersFormatFromPath(t *testing.T) {
	tests := []struct {
		args       string
		wantFormat string
		wantPath   string
	}{
		{"maintainer-log.txt", "txt", "maintainer-log.txt"},
		{"session.md", "markdown", "session.md"},
		{"session.json", "json", "session.json"},
		{"--format markdown", "markdown", "session-abcdef.md"},
		{"--format=json maintainer.json", "json", "maintainer.json"},
	}

	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			format, path, err := parseExportArgs(tt.args, "abcdef")
			if err != nil {
				t.Fatalf("parseExportArgs(%q) error = %v", tt.args, err)
			}
			if format != tt.wantFormat {
				t.Fatalf("format = %q, want %q", format, tt.wantFormat)
			}
			if path != tt.wantPath {
				t.Fatalf("path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

func TestParseExportArgsRejectsUnknownFormat(t *testing.T) {
	_, _, err := parseExportArgs("--format html", "12345678")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestExportSessionWritesRedactedJSONToRequestedPath(t *testing.T) {
	session := state.NewSession("test-model")
	session.ID = "12345678-1234-1234-1234-123456789abc"
	session.AddMessage(types.Message{
		Role: types.RoleUser,
		Content: []types.ContentBlock{
			types.TextBlock{Text: "OPENROUTER_API_KEY=sk-or-v1-secret123"},
		},
	})

	path := filepath.Join(t.TempDir(), "maintainer.json")
	gotPath, err := exportSession(session, "--format json "+path)
	if err != nil {
		t.Fatalf("exportSession() error = %v", err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "<redacted>") {
		t.Fatalf("export did not contain redaction marker: %s", body)
	}
	if strings.Contains(body, "sk-or-v1-secret123") {
		t.Fatalf("export leaked secret: %s", body)
	}
}

func TestExportSessionInfersMarkdownFormatFromPath(t *testing.T) {
	session := state.NewSession("test-model")
	session.ID = "abcdef12-1234-1234-1234-123456789abc"
	session.AddMessage(types.Message{
		Role: types.RoleAssistant,
		Content: []types.ContentBlock{
			types.TextBlock{Text: "ready"},
		},
	})

	path := filepath.Join(t.TempDir(), "session.md")
	gotPath, err := exportSession(session, path)
	if err != nil {
		t.Fatalf("exportSession() error = %v", err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	if body := string(data); !strings.Contains(body, "# Session abcdef12") || !strings.Contains(body, "ready") {
		t.Fatalf("markdown export missing expected content: %s", body)
	}
}

func TestExportSessionReturnsSaveError(t *testing.T) {
	session := state.NewSession("test-model")
	session.ID = "12345678-1234-1234-1234-123456789abc"

	dirPath := t.TempDir()
	_, err := exportSession(session, "--format json "+dirPath)
	if err == nil {
		t.Fatal("expected error when export path is a directory")
	}
	if !strings.Contains(err.Error(), "failed to write export file") {
		t.Fatalf("error = %q, want write failure context", err)
	}
}
