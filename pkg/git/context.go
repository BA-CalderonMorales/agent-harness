// Git integration for project context
// Inspired by claw-code's git integration

package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context holds git context information
type Context struct {
	IsRepo     bool
	Root       string
	Branch     string
	Commit     string
	HasChanges bool
	RemoteURL  string
}

// GetContext retrieves git context for the current directory
func GetContext() (*Context, error) {
	ctx := &Context{}

	// Check if we're in a git repo
	root, err := getGitRoot()
	if err != nil {
		ctx.IsRepo = false
		return ctx, nil
	}

	ctx.IsRepo = true
	ctx.Root = root

	// Get branch
	if branch, err := getGitBranch(); err == nil {
		ctx.Branch = branch
	}

	// Get commit hash
	if commit, err := getGitCommit(); err == nil {
		ctx.Commit = commit
	}

	// Check for uncommitted changes
	ctx.HasChanges = hasUncommittedChanges()

	// Get remote URL
	if remote, err := getRemoteURL(); err == nil {
		ctx.RemoteURL = remote
	}

	return ctx, nil
}

// getGitRoot returns the git repository root
func getGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// getGitBranch returns the current git branch
func getGitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// getGitCommit returns the current commit hash (short)
func getGitCommit() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hasUncommittedChanges checks if there are uncommitted changes
func hasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// getRemoteURL returns the origin remote URL
func getRemoteURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDiff returns the current git diff
func GetDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// FormatStatus returns formatted git status for display
func (ctx *Context) FormatStatus() string {
	if !ctx.IsRepo {
		return "Not a git repository"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%s", ctx.Root))

	if ctx.Branch != "" {
		parts = append(parts, fmt.Sprintf("(%s)", ctx.Branch))
	}

	if ctx.HasChanges {
		parts = append(parts, "*")
	}

	return strings.Join(parts, " ")
}

// FormatDiff returns a formatted diff report
func FormatDiff() string {
	diff, err := GetDiff()
	if err != nil {
		return ""
	}

	if diff == "" {
		return "No uncommitted changes."
	}

	return fmt.Sprintf("Changes:\n%s", diff)
}

// GetRelativePath returns the path relative to git root
func GetRelativePath(absPath string) (string, error) {
	root, err := getGitRoot()
	if err != nil {
		return absPath, err
	}

	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return absPath, err
	}

	return rel, nil
}

// IsIgnored checks if a path is gitignored
func IsIgnored(path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", path)
	err := cmd.Run()
	return err == nil
}

// GetRecentCommits returns recent commit messages
func GetRecentCommits(n int) ([]string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-n", fmt.Sprintf("%d", n))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines, nil
}
