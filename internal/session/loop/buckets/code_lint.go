package buckets

import (
	"context"
	"fmt"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/session/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleLint runs linter on code.
func (c *CodeBucket) handleLint(ctx loop.ExecutionContext) loop.LoopResult {
	path, _ := ctx.Input["path"].(string)
	if path == "" {
		path = "."
	}

	// Detect language from files
	lang := c.detectLanguage(path)
	config, ok := defaults.CodeLanguageTools[lang]
	if !ok {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unsupported_language", fmt.Sprintf("no linter configured for %s", lang)),
		}
	}

	// Run linter with timeout
	args := c.buildLintArgs(config, path)
	lintCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	cmd := exec.CommandContext(lintCtx, config.Linter, args...)
	cmd.Dir = c.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Many linters exit with non-zero when issues found
	isError := err != nil && result == ""

	// Parse issues
	issues := c.parseLintOutput(result, lang)
	if len(issues) > c.maxIssues {
		issues = issues[:c.maxIssues]
	}

	var summary string
	if len(issues) == 0 && !isError {
		summary = fmt.Sprintf("✓ No issues found in %s", path)
	} else if isError {
		summary = fmt.Sprintf("✗ Linter failed: %v", err)
	} else {
		summary = fmt.Sprintf("Found %d issues in %s:\n", len(issues), path)
		for _, issue := range issues {
			summary += fmt.Sprintf("  [%s] %s:%d: %s\n", issue.Severity, issue.File, issue.Line, issue.Message)
		}
	}

	return loop.LoopResult{
		Success: !isError && len(issues) == 0,
		Data:    issues,
		Error: func() loop.LoopError {
			if isError {
				return loop.WrapError("lint_failed", err)
			}
			if len(issues) > 0 {
				return loop.NewLoopError("issues_found", fmt.Sprintf("%d issues found", len(issues)))
			}
			return loop.LoopError{}
		}(),
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: summary, IsError: isError || len(issues) > 0}},
		}},
	}
}

// handleFormat runs formatter on code.
func (c *CodeBucket) handleFormat(ctx loop.ExecutionContext) loop.LoopResult {
	path, _ := ctx.Input["path"].(string)
	if path == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "path is required"),
		}
	}

	// Detect language
	lang := c.detectLanguage(path)
	config, ok := defaults.CodeLanguageTools[lang]
	if !ok {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unsupported_language", fmt.Sprintf("no formatter configured for %s", lang)),
		}
	}

	// Check for dry-run mode
	dryRun := false
	if d, ok := ctx.Input["dry_run"].(bool); ok {
		dryRun = d
	}

	// Run formatter with timeout
	args := []string{path}
	if dryRun {
		// Add dry-run flag if supported
		switch lang {
		case "go":
			args = []string{"-l", path} // list files that would change
		case "python":
			args = []string{"--check", path}
		}
	}

	fmtCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	cmd := exec.CommandContext(fmtCtx, config.Formatter, args...)
	cmd.Dir = c.basePath

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil && !dryRun {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("format_failed", err),
		}
	}

	var msg string
	if dryRun {
		if strings.TrimSpace(result) == "" {
			msg = fmt.Sprintf("✓ %s is already formatted", path)
		} else {
			msg = fmt.Sprintf("✗ %s needs formatting", path)
		}
	} else {
		if strings.TrimSpace(result) == "" {
			msg = fmt.Sprintf("✓ Formatted %s", path)
		} else {
			msg = fmt.Sprintf("✓ Formatted %s\n%s", path, result)
		}
	}

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: msg}},
		}},
	}
}

// handleAnalyze performs code analysis.
func (c *CodeBucket) handleAnalyze(ctx loop.ExecutionContext) loop.LoopResult {
	path, _ := ctx.Input["path"].(string)
	if path == "" {
		path = "."
	}

	// Get file stats
	cmd := exec.Command("find", path, "-type", "f", "-name", "*.go", "-o", "-name", "*.js", "-o", "-name", "*.py", "-o", "-name", "*.ts")
	cmd.Dir = c.basePath

	output, _ := cmd.Output()
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 1 && files[0] == "" {
		files = []string{}
	}

	// Count lines of code
	var totalLines int
	var fileCounts = make(map[string]int)

	for _, file := range files {
		if file == "" {
			continue
		}
		ext := filepath.Ext(file)
		wc := exec.Command("wc", "-l", file)
		wc.Dir = c.basePath
		out, _ := wc.Output()
		var lines int
		fmt.Sscanf(string(out), "%d", &lines)
		totalLines += lines
		fileCounts[ext]++
	}

	// Build analysis
	var analysis strings.Builder
	analysis.WriteString(fmt.Sprintf("Code Analysis for %s:\n\n", path))
	analysis.WriteString(fmt.Sprintf("Total files: %d\n", len(files)))
	analysis.WriteString(fmt.Sprintf("Total lines: %d\n\n", totalLines))

	if len(fileCounts) > 0 {
		analysis.WriteString("Files by type:\n")
		for ext, count := range fileCounts {
			analysis.WriteString(fmt.Sprintf("  %s: %d\n", ext, count))
		}
	}

	// Check for common files
	analysis.WriteString("\nProject indicators:\n")
	indicators := []struct {
		file string
		desc string
	}{
		{"go.mod", "Go module"},
		{"package.json", "Node.js project"},
		{"requirements.txt", "Python project"},
		{"Cargo.toml", "Rust project"},
		{"Gemfile", "Ruby project"},
		{"Dockerfile", "Dockerized"},
		{"docker-compose.yml", "Docker Compose"},
		{"Makefile", "Make build"},
		{"README.md", "Has README"},
		{"LICENSE", "Has license"},
		{".git", "Git repository"},
	}

	for _, ind := range indicators {
		cmd := exec.Command("test", "-f", ind.file)
		cmd.Dir = c.basePath
		if err := cmd.Run(); err == nil {
			analysis.WriteString(fmt.Sprintf("  ✓ %s\n", ind.desc))
		}
	}

	result := analysis.String()

	return loop.LoopResult{
		Success: true,
		Data: map[string]any{
			"files":   len(files),
			"lines":   totalLines,
			"by_type": fileCounts,
		},
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// detectLanguage detects the primary language of a path.
func (c *CodeBucket) detectLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	}

	// Try to detect from directory
	checkFile := func(name string) bool {
		cmd := exec.Command("test", "-f", name)
		cmd.Dir = c.basePath
		return cmd.Run() == nil
	}

	if checkFile("go.mod") {
		return "go"
	}
	if checkFile("package.json") {
		return "javascript"
	}
	if checkFile("requirements.txt") || checkFile("pyproject.toml") {
		return "python"
	}
	if checkFile("Cargo.toml") {
		return "rust"
	}
	if checkFile("Gemfile") {
		return "ruby"
	}

	return ""
}

// buildLintArgs builds linter arguments.
func (c *CodeBucket) buildLintArgs(config defaults.CodeToolConfig, path string) []string {
	// Language-specific arg building
	switch config.Linter {
	case "golangci-lint":
		return []string{"run", path}
	case "eslint":
		return []string{path}
	case "pylint":
		return []string{path}
	default:
		return []string{path}
	}
}

// parseLintOutput parses linter output into structured issues.
func (c *CodeBucket) parseLintOutput(output string, lang string) []LintIssue {
	var issues []LintIssue
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simple parsing - real implementation would be more sophisticated
		var issue LintIssue

		switch lang {
		case "go":
			// Try to parse "file:line:col: message" format
			if parts := strings.SplitN(line, ":", 4); len(parts) >= 3 {
				issue.File = parts[0]
				fmt.Sscanf(parts[1], "%d", &issue.Line)
				if len(parts) >= 4 {
					issue.Message = strings.TrimSpace(parts[3])
				}
				// Infer severity from message
				if strings.Contains(line, "error") {
					issue.Severity = defaults.CodeSeverityError
				} else {
					issue.Severity = defaults.CodeSeverityWarning
				}
			}
		default:
			// Generic: just store the line as message
			issue.Message = line
			issue.Severity = defaults.CodeSeverityWarning
		}

		if issue.Message != "" {
			issues = append(issues, issue)
		}
	}

	return issues
}

// LintIssue represents a code issue
