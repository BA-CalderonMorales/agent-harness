package record

import (
	"strings"
	"unicode"
)

// clip collapses whitespace to single spaces and cuts s to at most n runes,
// ending in an ellipsis when it had to cut. The result is always one line.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	if n == 1 {
		return string(r[:n])
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// clipTail is clip for paths: when it must cut, it keeps the end, because the
// file name is what identifies a path.
func clipTail(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[len(r)-max(n, 0):])
	}
	return "…" + string(r[len(r)-(n-1):])
}

// pushNewest appends v and keeps only the newest max items.
func pushNewest[T any](list []T, v T, max int) []T {
	list = append(list, v)
	if len(list) > max {
		list = list[len(list)-max:]
	}
	return list
}

// moveToEnd appends v, first removing any equal earlier entry, so a repeated
// value counts as the most recent. It keeps the newest max items.
func moveToEnd(list []string, v string, max int) []string {
	out := make([]string, 0, len(list)+1)
	for _, item := range list {
		if item != v {
			out = append(out, item)
		}
	}
	return pushNewest(out, v, max)
}

var constraintLeads = []string{"never", "do not", "don't", "don’t", "must", "only", "always"}

// constraintsIn extracts conservative constraint sentences: ones that start
// with Never, Do not, Don't, Must, Only or Always, matched case-insensitively
// on a word boundary. Callers pass user text only.
func constraintsIn(text string) []string {
	sentences := splitSentences(text)
	var out []string
	for _, sentence := range sentences {
		s := strings.TrimRight(strings.TrimLeft(strings.TrimSpace(sentence), "-*>• \t"), ".!? ")
		lower := strings.ToLower(s)
		for _, lead := range constraintLeads {
			if !strings.HasPrefix(lower, lead) {
				continue
			}
			rest := lower[len(lead):]
			if rest == "" || rest[0] == ' ' || rest[0] == ',' || rest[0] == ':' {
				out = append(out, clip(s, MaxConstraintLen))
				break
			}
		}
	}
	return out
}

// splitSentences cuts at newlines and semicolons, and at '.', '!' or '?' only
// when followed by whitespace or the end of the text, so periods inside file
// names and paths (config.yml, main.go, .env) stay in the sentence.
func splitSentences(text string) []string {
	r := []rune(text)
	var out []string
	start := 0
	for i, c := range r {
		cut := c == '\n' || c == ';'
		if c == '.' || c == '!' || c == '?' {
			cut = i+1 == len(r) || unicode.IsSpace(r[i+1])
		}
		if cut {
			out = append(out, string(r[start:i]))
			start = i + 1
		}
	}
	return append(out, string(r[start:]))
}
