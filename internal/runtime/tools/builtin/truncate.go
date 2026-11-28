package builtin

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Default max output sizes to prevent context window overflow
const (
	maxBashOutputChars   = 12000
	maxBashOutputLines   = 300
	maxReadOutputChars   = 15000
	maxReadOutputLines   = 400
	truncationMessageFmt = "\n\n[Output truncated: exceeded %d %s limit (%d %s shown)]"
)

// truncateBashOutput truncates bash output to safe limits.
func truncateBashOutput(output string) string {
	return truncateOutput(output, maxBashOutputChars, maxBashOutputLines, "chars", "lines")
}

// truncateReadOutput truncates file read output to safe limits.
func truncateReadOutput(output string) string {
	return truncateOutput(output, maxReadOutputChars, maxReadOutputLines, "chars", "lines")
}

// truncateOutput truncates by both character and line count, whichever is exceeded first.
func truncateOutput(output string, maxChars, maxLines int, unitChars, unitLines string) string {
	lines := strings.Split(output, "\n")
	truncatedByLines := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		output = strings.Join(lines, "\n")
		truncatedByLines = true
	}
	if len(output) > maxChars {
		cut := maxChars
		for cut > 0 && !utf8.RuneStart(output[cut]) {
			cut--
		}
		output = output[:cut]
		// Try to cut at last newline to avoid breaking mid-line
		if idx := strings.LastIndex(output, "\n"); idx > maxChars/2 {
			output = output[:idx]
		}
		output += fmt.Sprintf(truncationMessageFmt, maxChars, unitChars, maxChars, unitChars)
	} else if truncatedByLines {
		output += fmt.Sprintf(truncationMessageFmt, maxLines, unitLines, maxLines, unitLines)
	}
	return output
}
