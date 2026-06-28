package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveWorkspacePath(base, workspacePath string) (string, error) {
	if workspacePath == "" {
		return filepath.Clean(base), nil
	}
	resolved, err := resolveLocalPath(base, workspacePath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("%s: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return resolved, nil
}

func resolveLocalPath(base, path string) (string, error) {
	expanded := os.ExpandEnv(path)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("cannot expand %q without a home directory", path)
		}
		if expanded == "~" {
			expanded = home
		} else {
			expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
		}
	}
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(base, expanded)
	}
	return filepath.Clean(expanded), nil
}
