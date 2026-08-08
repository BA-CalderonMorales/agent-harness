// Package buckets provides domain-specific LoopBase implementations.

package buckets

import (
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop/buckets/defaults"
	"time"
)

// LoopCode handles code analysis operations.
// Tools: lint, format, analyze_code
type CodeBucket struct {
	basePath  string
	timeout   time.Duration
	maxIssues int
}

// NewLoopCode creates a code bucket.
func Code(basePath string) *CodeBucket {
	return &CodeBucket{
		basePath:  basePath,
		timeout:   defaults.CodeLintTimeoutSecs * time.Second,
		maxIssues: defaults.CodeLintMaxIssues,
	}
}

// WithTimeout sets the timeout.
func (c *CodeBucket) WithTimeout(d time.Duration) *CodeBucket {
	c.timeout = d
	return c
}

// WithMaxIssues sets max issues to report.
func (c *CodeBucket) WithMaxIssues(n int) *CodeBucket {
	c.maxIssues = n
	return c
}

// Name returns the bucket identifier.
func (c *CodeBucket) Name() string {
	return "code"
}

// CanHandle determines if this bucket handles the tool.
func (c *CodeBucket) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "lint", "format", "analyze_code", "code_review":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (c *CodeBucket) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,
		IsReadOnly:        true,
		IsDestructive:     false,
		ToolNames:         []string{"lint", "format", "analyze_code", "code_review"},
		Category:          "code",
	}
}

// Execute runs the code operation.
func (c *CodeBucket) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "lint":
		return c.handleLint(ctx)
	case "format":
		return c.handleFormat(ctx)
	case "analyze_code", "code_review":
		return c.handleAnalyze(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "code bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

type LintIssue struct {
	File     string
	Line     int
	Column   int
	Severity string
	Message  string
	Code     string
}

// Ensure LoopCode implements LoopBase
var _ loop.LoopBase = (*CodeBucket)(nil)
