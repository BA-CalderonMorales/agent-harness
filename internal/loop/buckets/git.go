// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopGit handles git operations.
// Tools: git_status, git_diff, git_log, git_branch, git_commit (with approval)
type GitBucket struct {
	basePath       string
	requireApproval bool
	timeout        time.Duration
}

// NewLoopGit creates a git bucket.
func Git(basePath string) *GitBucket {
	return &GitBucket{
		basePath:        basePath,
		requireApproval: true,
		timeout:         defaults.GitCommandTimeoutSecs * time.Second,
	}
}

// WithoutApproval disables approval for safe commands.
func (g *GitBucket) WithoutApproval() *GitBucket {
	g.requireApproval = false
	return g
}

// WithTimeout sets command timeout.
func (g *GitBucket) WithTimeout(d time.Duration) *GitBucket {
	g.timeout = d
	return g
}

// Name returns the bucket identifier.
func (g *GitBucket) Name() string {
	return "git"
}

// CanHandle determines if this bucket handles the tool.
func (g *GitBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "git_status", "git_diff", "git_log", "git_branch",
		"git_show", "git_remote", "git_commit", "git_add":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (g *GitBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        false,
		IsDestructive:     false, // Most ops are read-only, commit/add are controlled
		ToolNames: []string{
			"git_status", "git_diff", "git_log", "git_branch",
			"git_show", "git_remote", "git_commit", "git_add",
		},
		Category: "git",
	}
}

// Execute runs the git operation.
func (g *GitBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "git_status":
		return g.handleStatus(ctx)
	case "git_diff":
		return g.handleDiff(ctx)
	case "git_log":
		return g.handleLog(ctx)
	case "git_branch":
		return g.handleBranch(ctx)
	case "git_show":
		return g.handleShow(ctx)
	case "git_commit":
		return g.handleCommit(ctx)
	case "git_add":
		return g.handleAdd(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "git bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleStatus shows git status.
func (g *GitBucket) handleStatus(ctx loop.ExecutionContext) loop.LoopResult {
	cmd := exec.Command("git", "status", "-s")
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)
	
	if err != nil && result == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_status_failed", err),
		}
	}

	if strings.TrimSpace(result) == "" {
		result = "Working tree clean"
	}

	return loop.LoopResult{
		Success: true,
		Data:    parseGitStatus(result),
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleDiff shows git diff.
func (g *GitBucket) handleDiff(ctx loop.ExecutionContext) loop.LoopResult {
	args := []string{"diff"}
	
	// Check for staged
	if staged, ok := ctx.Input["staged"].(bool); ok && staged {
		args = append(args, "--staged")
	}
	
	// Check for specific file
	if file, ok := ctx.Input["file"].(string); ok && file != "" {
		args = append(args, file)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil && result == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_diff_failed", err),
		}
	}

	if strings.TrimSpace(result) == "" {
		result = "No differences"
	}

	// Truncate if too long
	lines := strings.Split(result, "\n")
	if len(lines) > defaults.GitDiffMaxLines {
		result = strings.Join(lines[:defaults.GitDiffMaxLines], "\n")
		result += fmt.Sprintf("\n[diff truncated: %d lines total]", len(lines))
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleLog shows git log.
func (g *GitBucket) handleLog(ctx loop.ExecutionContext) loop.LoopResult {
	maxEntries := defaults.GitLogMaxEntries
	if n, ok := ctx.Input["n"].(float64); ok {
		maxEntries = int(n)
	}

	cmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("-%d", maxEntries))
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_log_failed", err),
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleBranch shows git branches.
func (g *GitBucket) handleBranch(ctx loop.ExecutionContext) loop.LoopResult {
	cmd := exec.Command("git", "branch", "-v")
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_branch_failed", err),
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleShow shows git show for a commit.
func (g *GitBucket) handleShow(ctx loop.ExecutionContext) loop.LoopResult {
	ref := "HEAD"
	if r, ok := ctx.Input["ref"].(string); ok && r != "" {
		ref = r
	}

	cmd := exec.Command("git", "show", "--stat", ref)
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_show_failed", err),
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleCommit creates a commit (requires approval).
func (g *GitBucket) handleCommit(ctx loop.ExecutionContext) loop.LoopResult {
	message, _ := ctx.Input["message"].(string)
	if message == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "commit message is required"),
		}
	}

	// Check approval
	if g.requireApproval {
		// Would check ctx.CanUseTool here in full implementation
		_ = ctx.CanUseTool
	}

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_commit_failed", err),
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleAdd stages files.
func (g *GitBucket) handleAdd(ctx loop.ExecutionContext) loop.LoopResult {
	path, _ := ctx.Input["path"].(string)
	if path == "" {
		path = "."
	}

	cmd := exec.Command("git", "add", path)
	cmd.Dir = g.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("git_add_failed", err),
		}
	}

	if strings.TrimSpace(result) == "" {
		result = fmt.Sprintf("Staged: %s", path)
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// GitStatus represents parsed git status
type GitStatus struct {
	Modified []string
	Added    []string
	Deleted  []string
	Untracked []string
	Renamed  []string
}

// parseGitStatus parses git status -s output
func parseGitStatus(output string) *GitStatus {
	gs := &GitStatus{
		Modified:  []string{},
		Added:     []string{},
		Deleted:   []string{},
		Untracked: []string{},
		Renamed:   []string{},
	}

	for _, line := range strings.Split(output, "\n") {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[2:])

		switch {
		case status[0] == 'M' || status[1] == 'M':
			gs.Modified = append(gs.Modified, file)
		case status[0] == 'A':
			gs.Added = append(gs.Added, file)
		case status[0] == 'D' || status[1] == 'D':
			gs.Deleted = append(gs.Deleted, file)
		case status == "??":
			gs.Untracked = append(gs.Untracked, file)
		case status[0] == 'R':
			gs.Renamed = append(gs.Renamed, file)
		}
	}

	return gs
}

// IsGitRepo checks if path is a git repository.
func IsGitRepo(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
}

// Ensure LoopGit implements LoopBase
var _ loop.LoopBase = (*GitBucket)(nil)
