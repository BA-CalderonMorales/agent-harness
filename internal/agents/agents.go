// Package agents loads user-authored sub-agent definitions.
//
// A definition is a Markdown file with YAML frontmatter: the body is the
// sub-agent's instructions, and the frontmatter names it and bounds it.
//
//	---
//	name: reviewer
//	description: Reviews a finished change for correctness.
//	model: inherit
//	tools: [read, grep, glob]
//	---
//	You are a code reviewer. Report findings with evidence.
//
// The agent tool's agent_type picks one of these, so a delegation can
// have its own instructions, its own tool set, and its own model
// instead of being a generic worker every time.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Definition is one sub-agent.
type Definition struct {
	Name        string
	Description string
	// Model is the model the sub-agent should use. Empty or "inherit"
	// keeps the parent's model.
	Model string
	// Tools is an optional allowlist of tool names. Empty means the
	// sub-agent inherits the parent's tools.
	Tools  []string
	Prompt string
	Path   string
}

// frontmatter is the YAML header of a definition file.
type frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       []string `yaml:"tools"`
}

// Registry holds the discovered definitions by name.
type Registry struct {
	defs map[string]Definition
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]Definition)}
}

// Register adds or replaces a definition.
func (r *Registry) Register(def Definition) {
	r.defs[def.Name] = def
}

// Get looks a definition up by name, case-insensitively.
func (r *Registry) Get(name string) (Definition, bool) {
	if def, ok := r.defs[name]; ok {
		return def, true
	}
	for _, def := range r.defs {
		if strings.EqualFold(def.Name, name) {
			return def, true
		}
	}
	return Definition{}, false
}

// All returns every definition, sorted by name.
func (r *Registry) All() []Definition {
	out := make([]Definition, 0, len(r.defs))
	for _, def := range r.defs {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Len returns the number of definitions.
func (r *Registry) Len() int { return len(r.defs) }

// LoadFromDirs discovers definitions across several roots. Later roots
// win, so a project overrides a user-level definition of the same name.
func LoadFromDirs(dirs ...string) (*Registry, error) {
	reg := NewRegistry()
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			def, err := LoadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			reg.Register(def)
		}
	}
	return reg, nil
}

// LoadFile parses one definition file.
func LoadFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	body := string(data)

	var fm frontmatter
	if strings.HasPrefix(body, "---") {
		rest := body[3:]
		rest = strings.TrimPrefix(rest, "\r")
		rest = strings.TrimPrefix(rest, "\n")
		end := strings.Index(rest, "\n---")
		if end < 0 {
			return Definition{}, fmt.Errorf("%s: frontmatter is not closed", path)
		}
		header := rest[:end]
		prompt := rest[end+4:]
		prompt = strings.TrimPrefix(prompt, "\r")
		prompt = strings.TrimPrefix(prompt, "\n")
		if err := yaml.Unmarshal([]byte(header), &fm); err != nil {
			return Definition{}, fmt.Errorf("%s: %w", path, err)
		}
		body = strings.TrimSpace(prompt)
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	return Definition{
		Name:        name,
		Description: strings.TrimSpace(fm.Description),
		Model:       strings.TrimSpace(fm.Model),
		Tools:       fm.Tools,
		Prompt:      strings.TrimSpace(body),
		Path:        path,
	}, nil
}
