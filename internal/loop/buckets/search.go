package buckets

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/loop"
	"github.com/BA-CalderonMorales/agent-harness/internal/loop/buckets/defaults"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// LoopSearch handles search operations (grep, web search, etc.).
// It implements LoopBase for all search-related tools.
type LoopSearch struct {
	basePath      string
	maxResults    int
	maxLineLength int
	contextLines  int // Lines of context around matches
}

// NewLoopSearch creates a search bucket.
func NewLoopSearch(basePath string) *LoopSearch {
	return &LoopSearch{
		basePath:      basePath,
		maxResults:    defaults.SearchMaxResultsDefault,
		maxLineLength: defaults.SearchMaxLineLength,
		contextLines:  defaults.SearchContextLinesDefault,
	}
}

// WithMaxResults sets the maximum number of results.
func (s *LoopSearch) WithMaxResults(n int) *LoopSearch {
	s.maxResults = n
	return s
}

// WithContextLines sets the lines of context around matches.
func (s *LoopSearch) WithContextLines(n int) *LoopSearch {
	s.contextLines = n
	return s
}

// Name returns the bucket identifier.
func (s *LoopSearch) Name() string {
	return "search"
}

// CanHandle determines if this bucket handles the tool.
func (s *LoopSearch) CanHandle(toolName string, input map[string]any) bool {
	switch toolName {
	case "grep", "search", "search_code", "find", "websearch", "web_search":
		return true
	}
	return false
}

// Capabilities describes what this bucket can do.
func (s *LoopSearch) Capabilities() loop.BucketCapabilities {
	return loop.BucketCapabilities{
		IsConcurrencySafe: true,  // Search is read-only
		IsReadOnly:        true,  // Doesn't modify anything
		IsDestructive:     false, // Safe
		ToolNames:         []string{"grep", "search", "search_code", "find", "websearch", "web_search"},
		Category:          "search",
	}
}

// Execute runs the search operation.
func (s *LoopSearch) Execute(ctx loop.ExecutionContext) loop.LoopResult {
	switch ctx.ToolName {
	case "grep", "search", "search_code":
		return s.handleGrep(ctx)
	case "find":
		return s.handleFind(ctx)
	case "websearch", "web_search":
		return s.handleWebSearch(ctx)
	default:
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("unknown_tool", "search bucket doesn't handle: "+ctx.ToolName),
		}
	}
}

// handleGrep runs a grep search.
func (s *LoopSearch) handleGrep(ctx loop.ExecutionContext) loop.LoopResult {
	pattern, ok := ctx.Input["pattern"].(string)
	if !ok || pattern == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "pattern is required"),
		}
	}

	path := "."
	if p, ok := ctx.Input["path"].(string); ok && p != "" {
		path = p
	}

	filePattern := ""
	if fp, ok := ctx.Input["file_pattern"].(string); ok {
		filePattern = fp
	}

	// Build grep command
	args := []string{"-rn"}
	if s.contextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", s.contextLines))
	}
	args = append(args, "--", pattern, path)

	if filePattern != "" {
		args = append(args, "--include", filePattern)
	}

	cmd := exec.Command("grep", args...)
	cmd.Dir = s.basePath

	output, err := cmd.CombinedOutput()

	// grep exits with code 1 if no matches found - this is not an error
	if err != nil && !isGrepNoMatch(err, output) {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("grep_failed", err),
		}
	}

	results := s.processGrepOutput(string(output))

	if len(results) == 0 {
		return loop.LoopResult{
			Success: true,
			Data:    "(no matches)",
			Messages: []types.Message{{
				Role:    types.RoleUser,
				Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: "(no matches)"}},
			}},
		}
	}

	result := strings.Join(results, "\n")
	if len(results) >= s.maxResults {
		result += fmt.Sprintf("\n[showing %d of many matches]", s.maxResults)
	}

	return loop.LoopResult{
		Success: true,
		Data:    results,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// handleFind finds files by name.
func (s *LoopSearch) handleFind(ctx loop.ExecutionContext) loop.LoopResult {
	name, ok := ctx.Input["name"].(string)
	if !ok || name == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "name is required"),
		}
	}

	path := "."
	if p, ok := ctx.Input["path"].(string); ok && p != "" {
		path = p
	}

	ftype := "f" // file by default
	if t, ok := ctx.Input["type"].(string); ok {
		ftype = t
	}

	cmd := exec.Command("find", path, "-type", ftype, "-name", name)
	cmd.Dir = s.basePath

	output, err := cmd.Output()
	if err != nil {
		return loop.LoopResult{
			Success: false,
			Error:   loop.WrapError("find_failed", err),
		}
	}

	results := strings.TrimSpace(string(output))
	if results == "" {
		results = "(no matches)"
	}

	return loop.LoopResult{
		Success: true,
		Data:    results,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: results}},
		}},
	}
}

// handleWebSearch is a placeholder for web search.
func (s *LoopSearch) handleWebSearch(ctx loop.ExecutionContext) loop.LoopResult {
	query, ok := ctx.Input["query"].(string)
	if !ok || query == "" {
		return loop.LoopResult{
			Success: false,
			Error:   loop.NewLoopError("invalid_input", "query is required"),
		}
	}

	// This is a simplified placeholder
	// Real implementation would call search APIs
	result := fmt.Sprintf("Web search for '%s' would happen here.\n[Implement with actual search API]", query)

	return loop.LoopResult{
		Success: true,
		Data:    result,
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.ToolResultBlock{ToolUseID: ctx.ToolUseID, Content: result}},
		}},
	}
}

// processGrepOutput parses and limits grep output.
func (s *LoopSearch) processGrepOutput(output string) []string {
	var results []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(output)))
	
	for scanner.Scan() && len(results) < s.maxResults {
		line := scanner.Text()
		if len(line) > s.maxLineLength {
			line = line[:s.maxLineLength] + " [truncated]"
		}
		results = append(results, line)
	}

	return results
}

// isGrepNoMatch checks if grep error is just "no matches found".
func isGrepNoMatch(err error, output []byte) bool {
	if err == nil {
		return false
	}
	// grep exits with code 1 when no matches found
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() == 1 && len(output) == 0
	}
	return false
}

// SearchResult represents a parsed grep match.
type SearchResult struct {
	File    string
	Line    int
	Content string
	Context []string // Context lines
}

// IsCodeFile checks if file is a code file.
func IsCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, codeExt := range defaults.SearchCodeExtensions {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// ParseGrepResults parses grep output into structured results.
func ParseGrepResults(output string) []SearchResult {
	var results []SearchResult
	lines := strings.Split(output, "\n")
	
	resultMap := make(map[string]*SearchResult)
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// Parse grep -n output: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		
		file := parts[0]
		lineNum := 0
		fmt.Sscanf(parts[1], "%d", &lineNum)
		content := parts[2]
		
		key := fmt.Sprintf("%s:%d", file, lineNum)
		if _, exists := resultMap[key]; !exists {
			resultMap[key] = &SearchResult{
				File:    file,
				Line:    lineNum,
				Content: content,
			}
			results = append(results, *resultMap[key])
		}
	}
	
	return results
}

// PatternBuilder helps construct safe search patterns.
type PatternBuilder struct {
	terms []string
}

// NewPatternBuilder creates a pattern builder.
func NewPatternBuilder() *PatternBuilder {
	return &PatternBuilder{terms: make([]string, 0)}
}

// Add adds a term to the pattern.
func (pb *PatternBuilder) Add(term string) *PatternBuilder {
	// Escape special regex characters
	escaped := regexp.QuoteMeta(term)
	pb.terms = append(pb.terms, escaped)
	return pb
}

// Build creates the final pattern.
func (pb *PatternBuilder) Build() string {
	return strings.Join(pb.terms, ".*")
}

// Ensure LoopSearch implements LoopBase
var _ loop.LoopBase = (*LoopSearch)(nil)
