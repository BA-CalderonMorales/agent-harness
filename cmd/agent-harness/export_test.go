package main

import "testing"

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
