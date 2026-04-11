// Package buckets provides domain-specific LoopBase implementations.
package buckets

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopTest handles test execution.
// Tools: run_tests, test
type LoopTest struct {
	basePath   string
	timeout    time.Duration
	maxOutput  int
	parallel   int
}

// NewLoopTest creates a test bucket.
func NewLoopTest(basePath string) *LoopTest {
	return &LoopTest{
		basePath:  basePath,
		timeout:   defaults.TestDefaultTimeout,
		maxOutput: defaults.TestMaxOutputLines,
		parallel:  defaults.TestDefaultParallel,
	}
}

// WithTimeout sets the timeout.
func (t *LoopTest) WithTimeout(d time.Duration) *LoopTest {
	t.timeout = d
	return t
}

// WithParallel sets parallelism.
func (t *LoopTest) WithParallel(n int) *LoopTest {
	t.parallel = n
	return t
}

// Name returns the bucket identifier.
func (t *LoopTest) Name() string {
	return "test"
}

// CanHandle determines if this bucket handles the tool.
func (t *LoopTest) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "run_tests", "test", "run_test":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (t *LoopTest) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: false, // Tests may modify files
		IsReadOnly:        false,
		IsDestructive:     false,
		ToolNames:         []string{"run_tests", "test", "run_test"},
		Category:          "test",
	}
}

// Execute runs the test operation.
func (t *LoopTest) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "run_tests", "test", "run_test":
		return t.handleRunTests(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "test bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleRunTests runs tests.
func (t *LoopTest) handleRunTests(ctx loop.ExecutionContext) loop.LoopResult {
	// Detect test framework
	framework := t.detectTestFramework()
	config, ok := defaults.TestFrameworks[framework]
	if !ok {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unsupported_framework", fmt.Sprintf("no test framework for %s", framework)),
		}
	}

	// Build command
	pattern, _ := ctx.Input["pattern"].(string)
	coverage := false
	if cov, ok := ctx.Input["coverage"].(bool); ok {
		coverage = cov
	}

	args := t.buildTestArgs(config, pattern, coverage)

	// Run tests with timeout
	testCtx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	cmd := exec.CommandContext(testCtx, config.Command, args...)
	cmd.Dir = t.basePath

	if ctx.OnProgress != nil {
		ctx.OnProgress(map[string]any{"status": "running_tests"})
	}

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Truncate if too long
	lines := strings.Split(result, "\n")
	if len(lines) > t.maxOutput {
		lines = lines[:t.maxOutput]
		lines = append(lines, fmt.Sprintf("[output truncated: %d lines total]", len(strings.Split(result, "\n"))))
		result = strings.Join(lines, "\n")
	}

	// Parse results
	testResult := t.parseTestOutput(result, framework)

	if ctx.OnProgress != nil {
		ctx.OnProgress(map[string]any{
			"status": "complete",
			"passed": testResult.Passed,
			"failed": testResult.Failed,
		})
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Test Results (%s):\n", framework))
	summary.WriteString(fmt.Sprintf("  Passed: %d\n", testResult.Passed))
	summary.WriteString(fmt.Sprintf("  Failed: %d\n", testResult.Failed))
	summary.WriteString(fmt.Sprintf("  Skipped: %d\n", testResult.Skipped))
	if testResult.Coverage > 0 {
		summary.WriteString(fmt.Sprintf("  Coverage: %.1f%%\n", testResult.Coverage))
	}
	summary.WriteString("\n")

	// Show failures first
	if len(testResult.Failures) > 0 {
		summary.WriteString("Failures:\n")
		for _, fail := range testResult.Failures {
			summary.WriteString(fmt.Sprintf("  ✗ %s\n    %s\n", fail.Name, fail.Message))
		}
		summary.WriteString("\n")
	}

	// Show output preview
	summary.WriteString("Output preview:\n")
	outputLines := strings.Split(result, "\n")
	previewLines := 20
	if len(outputLines) < previewLines {
		previewLines = len(outputLines)
	}
	for _, line := range outputLines[len(outputLines)-previewLines:] {
		summary.WriteString(line + "\n")
	}

	success := err == nil && testResult.Failed == 0

	return loop.LoopResult{
		Success: success,
		Data:    testResult,
		Error: func() loop.LoopError {
			if !success {
				return loop.NewLoopError("tests_failed", fmt.Sprintf("%d tests failed", testResult.Failed))
			}
			return loop.LoopError{}
		}(),
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{
				ToolUseID: ctx.ToolUseID,
				Content:   summary.String(),
				IsError:   !success,
			}},
		}},
	}
}

// detectTestFramework detects the test framework.
func (t *LoopTest) detectTestFramework() string {
	// Check for go.mod
	if _, err := exec.LookPath("go"); err == nil {
		if _, err := exec.Command("test", "-f", filepath.Join(t.basePath, "go.mod")).Output(); err == nil {
			return "go"
		}
	}

	// Check for package.json
	if _, err := exec.LookPath("npm"); err == nil {
		if _, err := exec.Command("test", "-f", filepath.Join(t.basePath, "package.json")).Output(); err == nil {
			// Check if TypeScript
			if _, err := exec.Command("test", "-f", filepath.Join(t.basePath, "tsconfig.json")).Output(); err == nil {
				return "typescript"
			}
			return "javascript"
		}
	}

	// Check for Python
	if _, err := exec.LookPath("pytest"); err == nil {
		return "python"
	}
	if _, err := exec.LookPath("python"); err == nil {
		if _, err := exec.Command("test", "-f", filepath.Join(t.basePath, "requirements.txt")).Output(); err == nil {
			return "python"
		}
	}

	// Check for Rust
	if _, err := exec.LookPath("cargo"); err == nil {
		if _, err := exec.Command("test", "-f", filepath.Join(t.basePath, "Cargo.toml")).Output(); err == nil {
			return "rust"
		}
	}

	// Check for Ruby
	if _, err := exec.LookPath("rspec"); err == nil {
		return "ruby"
	}

	return ""
}

// buildTestArgs builds test command arguments.
func (t *LoopTest) buildTestArgs(config defaults.TestFrameworkConfig, pattern string, coverage bool) []string {
	args := []string{}

	// Add base args
	if config.Args != "" {
		args = append(args, config.Args)
	}

	// Add pattern if specified
	if pattern != "" && config.PatternArg != "" {
		args = append(args, config.PatternArg, pattern)
	}

	// Add coverage if requested
	if coverage && config.CoverageArg != "" {
		args = append(args, config.CoverageArg)
	}

	// Add timeout if specified
	if config.TimeoutArg != "" {
		args = append(args, config.TimeoutArg, fmt.Sprintf("%ds", int(t.timeout.Seconds())))
	}

	// Add parallel if specified
	if config.ParallelArg != "" && t.parallel > 1 {
		args = append(args, config.ParallelArg, fmt.Sprintf("%d", t.parallel))
	}

	return args
}

// parseTestOutput parses test output.
func (t *LoopTest) parseTestOutput(output string, framework string) *TestResult {
	result := &TestResult{
		Passed:   0,
		Failed:   0,
		Skipped:  0,
		Failures: []TestFailure{},
	}

	switch framework {
	case "go":
		// Parse "PASS", "FAIL", "SKIP"
		passRe := regexp.MustCompile(`^PASS|^ok`)
		failRe := regexp.MustCompile(`^FAIL|--- FAIL`)
		skipRe := regexp.MustCompile(`^SKIP|--- SKIP`)
		covRe := regexp.MustCompile(`coverage:\s+([\d.]+)%`)

		for _, line := range strings.Split(output, "\n") {
			if passRe.MatchString(line) {
				result.Passed++
			} else if failRe.MatchString(line) {
				result.Failed++
				// Extract test name
				if parts := strings.Split(line, " "); len(parts) > 2 {
					result.Failures = append(result.Failures, TestFailure{
						Name:    parts[2],
						Message: "Test failed",
					})
				}
			} else if skipRe.MatchString(line) {
				result.Skipped++
			}

			// Parse coverage
			if match := covRe.FindStringSubmatch(line); match != nil {
				fmt.Sscanf(match[1], "%f", &result.Coverage)
			}
		}

	default:
		// Generic parsing - look for common patterns
		passRe := regexp.MustCompile(`(?i)(\d+)\s+passed`)
		failRe := regexp.MustCompile(`(?i)(\d+)\s+failed`)

		for _, line := range strings.Split(output, "\n") {
			if match := passRe.FindStringSubmatch(line); match != nil {
				fmt.Sscanf(match[1], "%d", &result.Passed)
			}
			if match := failRe.FindStringSubmatch(line); match != nil {
				fmt.Sscanf(match[1], "%d", &result.Failed)
			}
		}
	}

	return result
}

// TestResult holds test execution results
type TestResult struct {
	Passed   int
	Failed   int
	Skipped  int
	Coverage float64
	Failures []TestFailure
	Duration time.Duration
}

// TestFailure represents a failed test
type TestFailure struct {
	Name    string
	Message string
	Output  string
}

// Ensure LoopTest implements LoopBase
var _ loop.LoopBase = (*LoopTest)(nil)
