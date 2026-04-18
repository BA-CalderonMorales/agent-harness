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
	IsRepo        bool
	Root          string
	Branch        string
	Commit        string
	HasChanges    bool
	RemoteURL     string
	RecentCommits []string
	StatusFiles   []string
	TopLevelFiles []string
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

	// Rich context: recent commits
	if commits, err := getRecentCommits(3); err == nil {
		ctx.RecentCommits = commits
	}

	// Rich context: status files (capped)
	if status, err := getStatusShort(20); err == nil {
		ctx.StatusFiles = status
	}

	// Rich context: top-level files (capped)
	if files, err := getTopLevelFiles(15); err == nil {
		ctx.TopLevelFiles = files
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

// getRecentCommits returns the last n commit messages (one-line)
func getRecentCommits(n int) ([]string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-n", fmt.Sprintf("%d", n))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}
	return lines, nil
}

// getStatusShort returns git status --short lines capped at max
func getStatusShort(max int) ([]string, error) {
	cmd := exec.Command("git", "status", "--short")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	all := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(all) == 1 && all[0] == "" {
		return []string{}, nil
	}
	if len(all) > max {
		all = all[:max]
		all = append(all, "...")
	}
	return all, nil
}

// getTopLevelFiles returns top-level non-hidden files/dirs capped at max
func getTopLevelFiles(max int) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// Fallback to ls if not a git repo or no HEAD
		return nil, err
	}
	all := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(all) == 1 && all[0] == "" {
		return []string{}, nil
	}
	if len(all) > max {
		all = all[:max]
		all = append(all, "...")
	}
	return all, nil
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
	parts = append(parts, ctx.Root)

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
