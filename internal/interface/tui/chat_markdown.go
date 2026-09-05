package tui

import (
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/diag"
)

var (
	markdownRenderersMu sync.Mutex
	markdownRenderers   = map[int]*glamour.TermRenderer{}
)

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
	// Headers carry weight by decoration, not just hue: H2 underlined,
	// H3 prefixed — the hierarchy survives greyscale themes and eye
	// fatigue alike.
	style.H1.Color = &primary
	style.H1.Bold = boolPtr(true)
	h1Prefix := "▎"
	style.H1.BlockPrefix = h1Prefix
	style.H2.Color = &primary
	style.H2.Bold = boolPtr(true)
	style.H2.Underline = boolPtr(true)
	style.H3.Color = &text
	style.H3.Bold = boolPtr(true)
	style.Strong.Bold = boolPtr(true)
	style.Link.Color = &dim
	style.Link.Underline = boolPtr(true)
	style.LinkText.Color = &text
	style.Code.Color = &text
	style.BlockQuote.Prefix = strings.Repeat(" ", 0) + "│ "
	style.BlockQuote.Color = &dim

	// Tables read as tables: explicit rules between rows and columns.
	center, column, row := "┼", "│", "─"
	style.Table.CenterSeparator = &center
	style.Table.ColumnSeparator = &column
	style.Table.RowSeparator = &row

	// Paragraphs breathe: one blank line between them. The margins were
	// stripped with every other block margin, but paragraph separation
	// is structure, not decoration — without it, multi-paragraph
	// responses run together into one wall of text.
	margin := uint(1)
	style.Paragraph.Margin = &margin

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

// evictMarkdownRenderer drops the cached renderer for a width so the
// next render builds a fresh one. Called after a glamour panic: a
// renderer that panicked mid-render carries dirty internal state.
func evictMarkdownRenderer(width int) {
	markdownRenderersMu.Lock()
	defer markdownRenderersMu.Unlock()
	delete(markdownRenderers, width)
}

// blockPrefixRE matches lines that start a markdown block: headings,
// lists, blockquotes, tables. Soft breaks never cross these.
var blockPrefixRE = regexp.MustCompile(`^(#{1,6}\s|[-*+]\s|\d+\.\s|>|\|)`)

// normalizeSoftBreaks collapses single newlines inside paragraphs into
// spaces, per markdown soft-break semantics. Models hard-wrap prose at
// arbitrary columns; glamour then renders each literal break as a line
// break, leaving ragged half-lines. Fenced code, tables, lists,
// headings, and quotes are structural and keep their newlines.
func normalizeSoftBreaks(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence || blockPrefixRE.MatchString(line) || strings.TrimSpace(line) == "" {
			out = append(out, line)
			continue
		}
		// Plain prose: join to the previous line when that line was also
		// plain prose (a soft break inside the same paragraph).
		if n := len(out); n > 0 && out[n-1] != "" && !blockPrefixRE.MatchString(out[n-1]) &&
			!strings.HasPrefix(strings.TrimSpace(out[n-1]), "```") {
			out[n-1] += " " + strings.TrimSpace(line)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// renderMarkdown converts markdown text to ANSI-styled text.
// A glamour panic on pathological input must never reach stderr: in a
// TUI, stderr interleaves with the rendered UI. It is logged to diag
// with a content snippet and the input falls through unstyled.
//
// The recovered panic also evicts the cached renderer for this width:
// a glamour panic mid-render leaves the TermRenderer's internal state
// dirty, and every later render sharing that instance panics too (the
// cascading "index out of range" storms in the diagnostics log).
func renderMarkdown(content string, width int) (result string) {
	defer func() {
		if r := recover(); r != nil {
			snippet := content
			if len(snippet) > 160 {
				snippet = snippet[:160] + "…"
			}
			snippet = strings.ReplaceAll(snippet, "\n", "\\n")
			diag.Errorf("tui.renderMarkdown.panic", "%v (width=%d content=%q)", r, width, snippet)
			evictMarkdownRenderer(width)
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
	rendered, err := renderer.Render(normalizeSoftBreaks(content))
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
