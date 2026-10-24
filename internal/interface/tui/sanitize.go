package tui

import "strings"

// Terminal sanitization.
//
// Model output and tool output are untrusted text that ends up in a
// terminal. A terminal interprets escape sequences: OSC 52 writes the
// clipboard, OSC 8 turns text into a hyperlink, CSI sequences move the
// cursor and erase the screen. A response that can emit those takes over
// the pane and can plant a fake prompt or steal the clipboard, and none
// of that needs a compromised harness.
//
// The filter keeps SGR (ESC [ ... m) so colored tool output still looks
// colored, and drops every other escape sequence: OSC, DCS, PM, APC,
// SOS, and non-SGR CSI. It also drops C0 controls other than newline and
// tab, plus the C1 range, which covers the 8-bit forms of CSI and OSC.

// SanitizeTerminalText removes terminal control sequences from text that
// came from the model, a tool, or a log line.
func SanitizeTerminalText(text string) string {
	if text == "" {
		return text
	}
	if !strings.ContainsRune(text, 0x1b) && !hasControlChar(text) {
		return text
	}

	runes := []rune(text)
	var out strings.Builder
	out.Grow(len(text))

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == 0x1b:
			i = skipEscapeSequence(runes, i, &out)
		case r == 0x9b:
			// 8-bit CSI.
			i = skipCSI(runes, i, i+1, &out)
		case r == 0x9d:
			// 8-bit OSC.
			i = skipStringSequence(runes, i+1)
		case r == 0x90:
			// 8-bit DCS.
			i = skipStringSequence(runes, i+1)
		case r == '\n' || r == '\t':
			out.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// Other C0 controls (bell, backspace, carriage return) and
			// delete: the terminal would act on them.
		case r >= 0x80 && r <= 0x9f:
			// Remaining C1 controls.
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// hasControlChar reports whether text carries anything the filter would
// touch, so the common case can return the input unchanged.
func hasControlChar(text string) bool {
	for _, r := range text {
		if r < 0x20 && r != '\n' && r != '\t' {
			return true
		}
		if r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return false
}

// skipEscapeSequence consumes the escape sequence starting at runes[i]
// (which is ESC) and returns the index of its last byte. SGR sequences
// are written back to out; everything else is dropped.
func skipEscapeSequence(runes []rune, i int, out *strings.Builder) int {
	if i+1 >= len(runes) {
		return i
	}
	switch runes[i+1] {
	case '[':
		// CSI: parameter bytes, then intermediate bytes, then a final
		// byte in 0x40-0x7E.
		return skipCSI(runes, i, i+2, out)
	case ']', 'P', 'X', '^', '_':
		// OSC, DCS, SOS, PM, APC: all run until BEL (OSC) or ST.
		return skipStringSequence(runes, i+2)
	default:
		// Two-character escape (ESC c, ESC =, ESC ( ...): drop both.
		return i + 1
	}
}

// skipCSI consumes a control sequence whose body starts at bodyStart and
// returns the index of its final byte. SGR is kept; every other final
// byte is dropped.
func skipCSI(runes []rune, start, bodyStart int, out *strings.Builder) int {
	for j := bodyStart; j < len(runes); j++ {
		if runes[j] >= 0x40 && runes[j] <= 0x7e {
			if runes[j] == 'm' {
				// SGR: colors and attributes only. Keep it.
				out.WriteString(string(runes[start : j+1]))
			}
			return j
		}
	}
	// Unterminated: drop the rest.
	return len(runes) - 1
}

// skipStringSequence consumes an OSC, DCS, SOS, PM, or APC body and
// returns the index of its terminator (BEL or ST).
func skipStringSequence(runes []rune, from int) int {
	for j := from; j < len(runes); j++ {
		if runes[j] == 0x07 {
			return j
		}
		if runes[j] == 0x1b && j+1 < len(runes) && runes[j+1] == '\\' {
			return j + 1
		}
	}
	return len(runes) - 1
}

// sanitizeTerminalLines applies the filter to every line of a
// multi-line block.
func sanitizeTerminalLines(text string) string {
	if !strings.Contains(text, "\n") {
		return SanitizeTerminalText(text)
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = SanitizeTerminalText(line)
	}
	return strings.Join(lines, "\n")
}
