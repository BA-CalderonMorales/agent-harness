package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/state"
)

func exportSession(session *state.Session, args string) (string, error) {
	format, path, err := parseExportArgs(args, session.ID)
	if err != nil {
		return "", err
	}
	if err := session.SaveExportToFile(path, format); err != nil {
		return "", err
	}
	return path, nil
}

func parseExportArgs(args, sessionID string) (string, string, error) {
	fields := strings.Fields(args)
	format := "txt"
	path := ""
	explicitFormat := false

	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch {
		case field == "--format":
			if i+1 >= len(fields) {
				return "", "", fmt.Errorf("--format requires a value")
			}
			explicitFormat = true
			format = fields[i+1]
			i++
		case strings.HasPrefix(field, "--format="):
			explicitFormat = true
			format = strings.TrimPrefix(field, "--format=")
		case path == "":
			path = field
		default:
			return "", "", fmt.Errorf("unexpected export argument: %q", field)
		}
	}

	normalized, ext, err := normalizeExportFormat(format)
	if err != nil {
		return "", "", err
	}
	format = normalized

	if path != "" && !explicitFormat {
		if inferred, inferredExt, ok := inferExportFormat(path); ok {
			format = inferred
			ext = inferredExt
		}
	}
	if path == "" {
		path = fmt.Sprintf("session-%s.%s", exportIDPrefix(sessionID), ext)
	}

	return format, path, nil
}

func normalizeExportFormat(format string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "txt", "text":
		return "txt", "txt", nil
	case "md", "markdown":
		return "markdown", "md", nil
	case "json":
		return "json", "json", nil
	default:
		return "", "", fmt.Errorf("unsupported format: %q (supported: txt, text, json, markdown, md)", format)
	}
}

func inferExportFormat(path string) (string, string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".log":
		return "txt", "txt", true
	case ".md", ".markdown":
		return "markdown", "md", true
	case ".json":
		return "json", "json", true
	default:
		return "", "", false
	}
}

func exportIDPrefix(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
