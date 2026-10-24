package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a loadable skill or knowledge module.
type Skill struct {
	Name        string
	Description string
	Path        string
	Content     string
}

// Registry manages discovered skills.
type Registry struct {
	skills map[string]Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]Skill)}
}

// Register adds a skill.
func (r *Registry) Register(s Skill) {
	r.skills[s.Name] = s
}

// Get retrieves a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// All returns every registered skill, sorted by name. The order is
// stable on purpose: the skill list reaches the system prompt, and a
// stable prompt keeps provider-side caching intact.
func (r *Registry) All() []Skill {
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LoadFromDirs discovers skills across several roots. Later roots win:
// the project copy of a skill overrides the user-level one of the same
// name. A root that does not exist is skipped; any other read error is
// returned.
func LoadFromDirs(dirs ...string) (*Registry, error) {
	reg := NewRegistry()
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		sub, err := LoadFromDirectory(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, skill := range sub.All() {
			reg.Register(skill)
		}
	}
	return reg, nil
}

// LoadFromDirectory discovers skills in a directory.
// Each subdirectory containing a SKILL.md is treated as a skill.
func LoadFromDirectory(dir string) (*Registry, error) {
	reg := NewRegistry()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name())
		mdPath := filepath.Join(skillPath, "SKILL.md")
		if _, err := os.Stat(mdPath); err == nil {
			content, err := os.ReadFile(mdPath)
			if err != nil {
				continue
			}
			reg.Register(Skill{
				Name:        entry.Name(),
				Description: strings.TrimSpace(string(content)),
				Path:        skillPath,
				Content:     string(content),
			})
		}
	}

	return reg, nil
}

// FormatPrompt injects skill content into a prompt.
func (s Skill) FormatPrompt() string {
	return fmt.Sprintf("\n\n<skill name=\"%s\">\n%s\n</skill>", s.Name, s.Content)
}
