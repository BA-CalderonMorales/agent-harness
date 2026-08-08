package tui

import (
	"fmt"
	"github.com/charmbracelet/glamour"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	markdownRenderer     *glamour.TermRenderer
	markdownRendererOnce sync.Once
	markdownRendererErr  error
	isTermux             = detectTermux()
)

// detectTermux checks if we're running in Termux environment
func detectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// getMarkdownRenderer returns a shared glamour renderer instance
func getMarkdownRenderer() (*glamour.TermRenderer, error) {
	markdownRendererOnce.Do(func() {
		markdownRenderer, markdownRendererErr = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(0), // We'll handle wrapping separately
		)
	})
	return markdownRenderer, markdownRendererErr
}

// renderMarkdown converts markdown text to ANSI-styled text
// In Termux, this returns plain text to avoid performance issues
func renderMarkdown(content string, width int) (result string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[PANIC RECOVERED] renderMarkdown: %v\n", r)
			result = content
		}
	}()

	if strings.TrimSpace(content) == "" {
		return content
	}

	// Skip expensive glamour rendering in Termux for better performance, but
	// keep core markdown affordances visible in plain terminal styling.
	if isTermux {
		return renderTermuxMarkdown(content)
	}

	renderer, err := getMarkdownRenderer()
	if err != nil {
		// Fallback to plain text if renderer fails
		return content
	}

	// Render markdown to ANSI
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing newline that glamour adds
	rendered = strings.TrimSuffix(rendered, "\n")

	return rendered
}

var (
	fencedCodeRE = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9_+-]+)?\n(.*?)```")
	inlineCodeRE = regexp.MustCompile("`([^`\n]+)`")
	boldRE       = regexp.MustCompile(`\*\*([^*\n]+)\*\*|__([^_\n]+)__`)
	italicRE     = regexp.MustCompile(`\*([^*\n]+)\*|_([^_\n]+)_`)
)

func renderTermuxMarkdown(content string) string {
	content = fencedCodeRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := fencedCodeRE.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		lines := strings.Split(strings.Trim(parts[1], "\n"), "\n")
		for i, line := range lines {
			lines[i] = CodeBlockStyle.Render("  " + line)
		}
		return strings.Join(lines, "\n")
	})
	content = inlineCodeRE.ReplaceAllString(content, CodeInlineStyle.Render("$1"))
	content = boldRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := boldRE.FindStringSubmatch(match)
		for _, part := range parts[1:] {
			if part != "" {
				return MarkdownBoldStyle.Render(part)
			}
		}
		return match
	})
	content = italicRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := italicRE.FindStringSubmatch(match)
		for _, part := range parts[1:] {
			if part != "" {
				return MarkdownItalicStyle.Render(part)
			}
		}
		return match
	})
	return content
}

// NewChatModel creates a new chat model.
// UI FIX: Styled textarea with consistent background for better visual appeal
