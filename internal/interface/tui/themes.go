package tui

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// Themes: 20 scoped palettes over the design system's thirteen tokens.
// Adding or dropping a theme is a one-block change: one entry in the
// themes map. The default entry is the palette styles_colors.go shipped
// with, so the boot look never changed.

// Palette is the scoped color set every style derives from.
type Palette struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Info      lipgloss.Color
	Text      lipgloss.Color
	TextDim   lipgloss.Color
	Surface   lipgloss.Color
	Border    lipgloss.Color
	Muted     lipgloss.Color
	Highlight lipgloss.Color
}

// Theme pairs a palette with its settings-facing name.
type Theme struct {
	Name    string
	Palette Palette
}

func c(hex string) lipgloss.Color { return lipgloss.Color(hex) }

var themes = map[string]Palette{
	"default": {
		Primary: c("#B388FF"), Secondary: c("#80CBC4"), Accent: c("#FFD54F"),
		Success: c("#69F0AE"), Error: c("#FF5252"), Warning: c("#FFB74D"),
		Info: c("#64B5F6"), Text: c("#E0E0E0"), TextDim: c("#9E9E9E"),
		Surface: c("#1E1E2E"), Border: c("#3A3A4A"), Muted: c("#5A5A6A"),
		Highlight: c("#2A2A3E"),
	},
	"midnight": {
		Primary: c("#7C9EFF"), Secondary: c("#5FD4C4"), Accent: c("#F2C94C"),
		Success: c("#6FCF97"), Error: c("#EB5757"), Warning: c("#F2994A"),
		Info: c("#56CCF2"), Text: c("#DDE3F0"), TextDim: c("#8B93A7"),
		Surface: c("#12141F"), Border: c("#2A2F45"), Muted: c("#4A5164"),
		Highlight: c("#1C2033"),
	},
	"forest": {
		Primary: c("#66BB6A"), Secondary: c("#A0D8A8"), Accent: c("#DCE775"),
		Success: c("#81C784"), Error: c("#E57373"), Warning: c("#FFB74D"),
		Info: c("#4DD0E1"), Text: c("#E3EADF"), TextDim: c("#8FA08F"),
		Surface: c("#15221A"), Border: c("#2C4234"), Muted: c("#4F6656"),
		Highlight: c("#1E3125"),
	},
	"solarized": {
		Primary: c("#268BD2"), Secondary: c("#2AA198"), Accent: c("#B58900"),
		Success: c("#859900"), Error: c("#DC322F"), Warning: c("#CB4B16"),
		Info: c("#6C71C4"), Text: c("#EEE8D5"), TextDim: c("#93A1A1"),
		Surface: c("#002B36"), Border: c("#073642"), Muted: c("#586E75"),
		Highlight: c("#073642"),
	},
	"gruvbox": {
		Primary: c("#83A598"), Secondary: c("#8EC07C"), Accent: c("#FABD2F"),
		Success: c("#B8BB26"), Error: c("#FB4934"), Warning: c("#FE8019"),
		Info: c("#83A598"), Text: c("#EBDBB2"), TextDim: c("#928374"),
		Surface: c("#282828"), Border: c("#504945"), Muted: c("#665C54"),
		Highlight: c("#3C3836"),
	},
	"nord": {
		Primary: c("#88C0D0"), Secondary: c("#8FBCBB"), Accent: c("#EBCB8B"),
		Success: c("#A3BE8C"), Error: c("#BF616A"), Warning: c("#D08770"),
		Info: c("#81A1C1"), Text: c("#ECEFF4"), TextDim: c("#7B88A1"),
		Surface: c("#2E3440"), Border: c("#434C5E"), Muted: c("#4C566A"),
		Highlight: c("#3B4252"),
	},
	"dracula": {
		Primary: c("#BD93F9"), Secondary: c("#50FA7B"), Accent: c("#F1FA8C"),
		Success: c("#50FA7B"), Error: c("#FF5555"), Warning: c("#FFB86C"),
		Info: c("#8BE9FD"), Text: c("#F8F8F2"), TextDim: c("#6272A4"),
		Surface: c("#282A36"), Border: c("#44475A"), Muted: c("#6272A4"),
		Highlight: c("#44475A"),
	},
	"monokai": {
		Primary: c("#F92672"), Secondary: c("#A6E22E"), Accent: c("#E6DB74"),
		Success: c("#A6E22E"), Error: c("#F92672"), Warning: c("#FD971F"),
		Info: c("#66D9EF"), Text: c("#F8F8F2"), TextDim: c("#75715E"),
		Surface: c("#272822"), Border: c("#49483E"), Muted: c("#75715E"),
		Highlight: c("#3E3D32"),
	},
	"tokyo-night": {
		Primary: c("#7AA2F7"), Secondary: c("#1ABC9C"), Accent: c("#E0AF68"),
		Success: c("#9ECE6A"), Error: c("#F7768E"), Warning: c("#E0AF68"),
		Info: c("#7DCFFF"), Text: c("#C0CAF5"), TextDim: c("#565F89"),
		Surface: c("#1A1B26"), Border: c("#292E42"), Muted: c("#414868"),
		Highlight: c("#24283B"),
	},
	"catppuccin": {
		Primary: c("#CBA6F7"), Secondary: c("#94E2D5"), Accent: c("#F9E2AF"),
		Success: c("#A6E3A1"), Error: c("#F38BA8"), Warning: c("#FAB387"),
		Info: c("#89DCEB"), Text: c("#CDD6F4"), TextDim: c("#6C7086"),
		Surface: c("#1E1E2E"), Border: c("#313244"), Muted: c("#7F849C"),
		Highlight: c("#45475A"),
	},
	"rose-pine": {
		Primary: c("#C4A7E7"), Secondary: c("#9CCFD8"), Accent: c("#F6C177"),
		Success: c("#31748F"), Error: c("#EB6F92"), Warning: c("#F6C177"),
		Info: c("#9CCFD8"), Text: c("#E0FAFA"), TextDim: c("#6F6C7A"),
		Surface: c("#191724"), Border: c("#26233A"), Muted: c("#555169"),
		Highlight: c("#1F1D2E"),
	},
	"everforest": {
		Primary: c("#A7C080"), Secondary: c("#83C092"), Accent: c("#DBBC7F"),
		Success: c("#83C092"), Error: c("#E67E80"), Warning: c("#D699B6"),
		Info: c("#7FBBB3"), Text: c("#D3C6AA"), TextDim: c("#859289"),
		Surface: c("#2D353B"), Border: c("#475258"), Muted: c("#5C6370"),
		Highlight: c("#3D484D"),
	},
	"kanagawa": {
		Primary: c("#7E9CD8"), Secondary: c("#98BB6C"), Accent: c("#FF9E3B"),
		Success: c("#98BB6C"), Error: c("#E82424"), Warning: c("#DCA561"),
		Info: c("#658594"), Text: c("#DCD7BA"), TextDim: c("#727169"),
		Surface: c("#1F1F28"), Border: c("#2A2A37"), Muted: c("#54546D"),
		Highlight: c("#363646"),
	},
	"one-dark": {
		Primary: c("#61AFEF"), Secondary: c("#56B6C2"), Accent: c("#E5C07B"),
		Success: c("#98C379"), Error: c("#E06C75"), Warning: c("#D19A66"),
		Info: c("#56B6C2"), Text: c("#ABB2BF"), TextDim: c("#5C6370"),
		Surface: c("#282C34"), Border: c("#3E4451"), Muted: c("#5C6370"),
		Highlight: c("#3E4451"),
	},
	"github-dark": {
		Primary: c("#58A6FF"), Secondary: c("#39C5CF"), Accent: c("#D29922"),
		Success: c("#3FB950"), Error: c("#F85149"), Warning: c("#D29922"),
		Info: c("#58A6FF"), Text: c("#C9D1D9"), TextDim: c("#8B949E"),
		Surface: c("#0D1117"), Border: c("#30363D"), Muted: c("#6E7681"),
		Highlight: c("#161B22"),
	},
	"cobalt": {
		Primary: c("#84D0FF"), Secondary: c("#3DDDE2"), Accent: c("#FFFA6A"),
		Success: c("#3DDDE2"), Error: c("#F14481"), Warning: c("#FF8A3D"),
		Info: c("#68A1F8"), Text: c("#DCF1FF"), TextDim: c("#5B7B93"),
		Surface: c("#0F1B44"), Border: c("#1C3A6B"), Muted: c("#3B5B8C"),
		Highlight: c("#14295A"),
	},
	"paper": {
		Primary: c("#4A6FA5"), Secondary: c("#5B8770"), Accent: c("#B08947"),
		Success: c("#5B8770"), Error: c("#A54444"), Warning: c("#B08947"),
		Info: c("#4A6FA5"), Text: c("#2B2B28"), TextDim: c("#8A8578"),
		Surface: c("#F5F2EA"), Border: c("#D4CFC1"), Muted: c("#A9A292"),
		Highlight: c("#EAE6DA"),
	},
	"moss": {
		Primary: c("#9CBF6E"), Secondary: c("#77A2A8"), Accent: c("#D9B36C"),
		Success: c("#77A2A8"), Error: c("#C0653A"), Warning: c("#D9B36C"),
		Info: c("#8FA876"), Text: c("#E2E4DA"), TextDim: c("#8B9282"),
		Surface: c("#232822"), Border: c("#3C4438"), Muted: c("#616B5B"),
		Highlight: c("#2E362C"),
	},
	"ember": {
		Primary: c("#E8985E"), Secondary: c("#C4826B"), Accent: c("#EBC06D"),
		Success: c("#96A85B"), Error: c("#D6524B"), Warning: c("#EBC06D"),
		Info: c("#7F9DBA"), Text: c("#E7DFD6"), TextDim: c("#948A80"),
		Surface: c("#241C1A"), Border: c("#40332F"), Muted: c("#6B5B54"),
		Highlight: c("#322622"),
	},
	"ice": {
		Primary: c("#6FB7D9"), Secondary: c("#87CEEB"), Accent: c("#A8DADC"),
		Success: c("#7FC8A9"), Error: c("#D98282"), Warning: c("#E5B769"),
		Info: c("#6FB7D9"), Text: c("#E8F1F4"), TextDim: c("#8FA6B0"),
		Surface: c("#1B2733"), Border: c("#31404F"), Muted: c("#556B7A"),
		Highlight: c("#243447"),
	},
}

// ThemeNames returns the theme names sorted alphabetically, default
// first so the catalog's face is always /theme's first suggestion.
func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		if name == "default" && i != 0 {
			names[0], names[i] = names[i], names[0]
			break
		}
	}
	return names
}

// LookupTheme resolves a theme name (case-insensitive, hyphen/underscore
// agnostic) with ok=false when unknown.
func LookupTheme(name string) (Theme, bool) {
	return lookupTheme(name)
}

func lookupTheme(name string) (Theme, bool) {
	norm := normalizeThemeName(name)
	for themeName, palette := range themes {
		if normalizeThemeName(themeName) == norm {
			return Theme{Name: themeName, Palette: palette}, true
		}
	}
	return Theme{}, false
}

func normalizeThemeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		out = append(out, lowerRune(r))
	}
	return string(out)
}

func lowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// ApplyTheme swaps the palette and rebuilds every derived style. The
// default theme restores the shipped palette exactly.
func ApplyTheme(name string) bool {
	theme, ok := lookupTheme(name)
	if !ok {
		return false
	}
	p := theme.Palette
	ColorPrimary, ColorSecondary, ColorAccent = p.Primary, p.Secondary, p.Accent
	ColorSuccess, ColorError, ColorWarning = p.Success, p.Error, p.Warning
	ColorInfo, ColorText, ColorTextDim = p.Info, p.Text, p.TextDim
	ColorSurface, ColorBorder, ColorMuted, ColorHighlight = p.Surface, p.Border, p.Muted, p.Highlight
	buildStyles()
	return true
}

// ApplyTheme applies the theme to the whole running app: the palette
// and styles, plus instance-captured colors the styles files can't
// reach (the composer caret). The package-level ApplyTheme only covers
// the vars.
func (a *App) ApplyTheme(name string) bool {
	ok := ApplyTheme(name)
	if ok {
		a.chatModel.refreshCursorStyle()
	}
	return ok
}
