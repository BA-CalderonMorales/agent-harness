package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadFileParsesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "reviewer.md", `---
name: reviewer
description: Reviews a finished change.
model: reviewer-model
tools: [read, grep]
---
You are a strict reviewer.
`)

	def, err := LoadFile(filepath.Join(dir, "reviewer.md"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if def.Name != "reviewer" {
		t.Fatalf("name = %q", def.Name)
	}
	if def.Description != "Reviews a finished change." {
		t.Fatalf("description = %q", def.Description)
	}
	if def.Model != "reviewer-model" {
		t.Fatalf("model = %q", def.Model)
	}
	if len(def.Tools) != 2 || def.Tools[0] != "read" || def.Tools[1] != "grep" {
		t.Fatalf("tools = %#v", def.Tools)
	}
	if def.Prompt != "You are a strict reviewer." {
		t.Fatalf("prompt = %q", def.Prompt)
	}
}

func TestLoadFileWithoutFrontmatterUsesTheFileName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "helper.md", "Just instructions.\n")

	def, err := LoadFile(filepath.Join(dir, "helper.md"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if def.Name != "helper" {
		t.Fatalf("name = %q, want the file name", def.Name)
	}
	if def.Prompt != "Just instructions." {
		t.Fatalf("prompt = %q", def.Prompt)
	}
}

func TestLoadFileRejectsUnclosedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.md", "---\nname: broken\nnever closed\n")

	if _, err := LoadFile(filepath.Join(dir, "broken.md")); err == nil {
		t.Fatal("LoadFile() = nil error, want a frontmatter error")
	}
}

func TestLoadFromDirsLaterRootWins(t *testing.T) {
	user := t.TempDir()
	project := t.TempDir()
	writeFile(t, user, "reviewer.md", "---\nname: reviewer\n---\nuser copy\n")
	writeFile(t, user, "helper.md", "---\nname: helper\n---\nuser helper\n")
	writeFile(t, project, "reviewer.md", "---\nname: reviewer\n---\nproject copy\n")

	registry, err := LoadFromDirs(user, project)
	if err != nil {
		t.Fatalf("LoadFromDirs() error = %v", err)
	}
	def, ok := registry.Get("reviewer")
	if !ok {
		t.Fatal("reviewer missing")
	}
	if def.Prompt != "project copy" {
		t.Fatalf("prompt = %q, want the project copy", def.Prompt)
	}
	if _, ok := registry.Get("helper"); !ok {
		t.Fatal("helper missing")
	}
	if registry.Len() != 2 {
		t.Fatalf("definitions = %d, want 2", registry.Len())
	}
}

func TestRegistryGetIsCaseInsensitive(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Definition{Name: "reviewer"})

	if _, ok := registry.Get("REVIEWER"); !ok {
		t.Fatal("Get(REVIEWER) missed the reviewer definition")
	}
}
