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

// HasGhCLI returns true if the GitHub CLI is available.
func HasGhCLI() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// CreatePR creates a pull request using gh CLI.
func (r *Repo) CreatePR(title, body string) (string, error) {
	args := []string{"pr", "create", "--title", title}
	if body != "" {
		args = append(args, "--body", body)
	} else {
		args = append(args, "--fill")
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = r.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// ListPRs lists open pull requests using gh CLI.
func (r *Repo) ListPRs() (string, error) {
	cmd := exec.Command("gh", "pr", "list", "--limit", "10")
	cmd.Dir = r.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr list failed: %s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// ListWorktrees returns all git worktrees.
func (r *Repo) ListWorktrees() (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "worktree", "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree list failed: %s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// AddWorktree creates a new worktree for the given branch.
func (r *Repo) AddWorktree(path, branch string) error {
	cmd := exec.Command("git", "-C", r.Path, "worktree", "add", path, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %s", string(out))
	}
	return nil
}

// RemoveWorktree removes a worktree at the given path.
func (r *Repo) RemoveWorktree(path string) error {
	cmd := exec.Command("git", "-C", r.Path, "worktree", "remove", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %s", string(out))
	}
	return nil
}
