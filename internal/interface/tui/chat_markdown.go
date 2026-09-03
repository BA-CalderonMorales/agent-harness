package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

var (
	markdownRenderersMu sync.Mutex
	markdownRenderers   = map[int]*glamour.TermRenderer{}
	isTermux            = detectTermux()
)

// detectTermux checks if we're running in Termux environment
func detectTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}

// transparentMarkdownStyle derives the app's glamour style from the
// dark base with one governing principle: the chat renders on the
// user's own terminal background, so the style may never paint one.
// Background fills, block margins, and chroma theme underlays are
// stripped; structure comes from color, weight, and gutters.
func transparentMarkdownStyle() ansi.StyleConfig {
	style := styles.DarkStyleConfig

	// Kill every background fill the dark base sets.
	transparent := func(p ansi.StylePrimitive) ansi.StylePrimitive {
		p.BackgroundColor = nil
		return p
	}
	stripBlock := func(b ansi.StyleBlock) ansi.StyleBlock {
		b.StylePrimitive = transparent(b.StylePrimitive)
		b.Margin = nil
		return b
	}
	style.Document = stripBlock(style.Document)
	style.Paragraph = stripBlock(style.Paragraph)
	style.Text = transparent(style.Text)
	style.Strong = transparent(style.Strong)
	style.Emph = transparent(style.Emph)
	style.Link = transparent(style.Link)
	style.LinkText = transparent(style.LinkText)
	style.Code = stripBlock(style.Code)
	style.BlockQuote = stripBlock(style.BlockQuote)
	style.Heading = stripBlock(style.Heading)
	style.H1 = stripBlock(style.H1)
	style.H2 = stripBlock(style.H2)
	style.H3 = stripBlock(style.H3)
	style.H4 = stripBlock(style.H4)
	style.H5 = stripBlock(style.H5)
	style.H6 = stripBlock(style.H6)
	style.List.StyleBlock = stripBlock(style.List.StyleBlock)
	style.Item = transparent(style.Item)
	style.Enumeration = transparent(style.Enumeration)

	// No block margins: the transcript's own bubble borders provide the
	// framing. Chroma theme "none" keeps syntax structure without a
	// code-block underlay.
	style.CodeBlock = ansi.StyleCodeBlock{
		StyleBlock: stripBlock(style.CodeBlock.StyleBlock),
		Theme:      "none",
	}

	// Palette-tuned structure.
	primary := string(ColorPrimary)
	text := string(ColorText)
	dim := string(ColorMuted)
	style.H1.Color = &primary
	style.H1.Bold = boolPtr(true)
	style.H2.Color = &primary
	style.H2.Bold = boolPtr(true)
	style.H3.Color = &text
	style.H3.Bold = boolPtr(true)
	style.Strong.Bold = boolPtr(true)
	style.Link.Color = &dim
	style.Link.Underline = boolPtr(true)
	style.LinkText.Color = &text
	style.Code.Color = &text
	style.BlockQuote.Prefix = strings.Repeat(" ", 0) + "│ "
	style.BlockQuote.Color = &dim

	return style
}

func boolPtr(b bool) *bool { return &b }

// getMarkdownRenderer returns a glamour renderer for a terminal width.
// Renderers cache per width: the style folds the wrap width in at
// construction, and resizes are rare.
func getMarkdownRenderer(width int) *glamour.TermRenderer {
	markdownRenderersMu.Lock()
	defer markdownRenderersMu.Unlock()
	if renderer, ok := markdownRenderers[width]; ok {
		return renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(transparentMarkdownStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	markdownRenderers[width] = renderer
	return renderer
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

	// Wrap at the bubble width: glamour indents and wraps per block, so
	// the width must be known at render time, not after.
	if width < 20 {
		width = 20
	}
	renderer := getMarkdownRenderer(width)
	if renderer == nil {
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
