package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Repo represents a git repository.
type Repo struct {
	Path string
}

// NewRepo creates a repo helper for the given path.
func NewRepo(path string) *Repo {
	return &Repo{Path: path}
}

// IsRepo returns true if the path is inside a git repository.
func (r *Repo) IsRepo() bool {
	cmd := exec.Command("git", "-C", r.Path, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// Status returns the git status as a string.
func (r *Repo) Status() (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "status", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}
	return string(out), nil
}

// CurrentBranch returns the current branch name.
func (r *Repo) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "branch", "--show-current")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Diff returns the working tree diff.
func (r *Repo) Diff() (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "diff")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Commit creates a commit with the given message.
func (r *Repo) Commit(message string) error {
	cmd := exec.Command("git", "-C", r.Path, "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", string(out))
	}
	return nil
}

// Add stages files.
func (r *Repo) Add(paths ...string) error {
	args := append([]string{"-C", r.Path, "add"}, paths...)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s", string(out))
	}
	return nil
}

// ListBranches returns all local branches with the current one marked.
func (r *Repo) ListBranches() ([]string, error) {
	cmd := exec.Command("git", "-C", r.Path, "branch")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %s", string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	return lines, nil
}

// CreateBranch creates a new branch and switches to it.
func (r *Repo) CreateBranch(name string) error {
	cmd := exec.Command("git", "-C", r.Path, "checkout", "-b", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %s", string(out))
	}
	return nil
}

// SwitchBranch switches to an existing branch.
func (r *Repo) SwitchBranch(name string) error {
	cmd := exec.Command("git", "-C", r.Path, "checkout", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %s", string(out))
	}
	return nil
}

// DeleteBranch deletes a local branch.
func (r *Repo) DeleteBranch(name string) error {
	cmd := exec.Command("git", "-C", r.Path, "branch", "-d", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch -d failed: %s", string(out))
	}
	return nil
}
