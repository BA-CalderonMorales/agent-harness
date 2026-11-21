package record

import "strings"

// shellSegment is one simple command and the operator that ended it.
type shellSegment struct {
	Text string
	Sep  string // "&&", "||", ";" or "|"; "" for the last segment
}

type heredoc struct {
	word  string
	strip bool
}

// splitShell is a small, pure scanner for the shape of a shell line. It splits
// at &&, ||, ;, | and newlines only outside quotes; it tracks single quotes,
// double quotes (with backslash escapes) and backslash-newline continuation;
// and it drops heredoc bodies, keeping only the `<<WORD` operator. Newlines
// inside quotes become one space. An unbalanced quote swallows the rest of the
// input and an unterminated heredoc swallows the rest of the lines; neither
// can panic or loop, because every step consumes at least one rune.
func splitShell(cmd string) []shellSegment {
	r := []rune(strings.ReplaceAll(cmd, "\r\n", "\n"))
	var segs []shellSegment
	var cur strings.Builder
	var pending []heredoc
	flush := func(sep string) {
		if text := strings.TrimSpace(cur.String()); text != "" {
			segs = append(segs, shellSegment{Text: text, Sep: sep})
		} else if sep != "" && len(segs) > 0 && segs[len(segs)-1].Sep == "" {
			segs[len(segs)-1].Sep = sep
		}
		cur.Reset()
	}
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r):
			if r[i+1] == '\n' {
				cur.WriteByte(' ')
			} else {
				cur.WriteRune(c)
				cur.WriteRune(r[i+1])
			}
			i += 2
		case c == '\'':
			cur.WriteRune(c)
			for i++; i < len(r) && r[i] != '\''; i++ {
				cur.WriteRune(spaceNL(r[i]))
			}
			if i < len(r) {
				cur.WriteRune(r[i])
				i++
			}
		case c == '"':
			cur.WriteRune(c)
			for i++; i < len(r) && r[i] != '"'; i++ {
				if r[i] == '\\' && i+1 < len(r) {
					cur.WriteRune(r[i])
					i++
				}
				cur.WriteRune(spaceNL(r[i]))
			}
			if i < len(r) {
				cur.WriteRune(r[i])
				i++
			}
		case c == '<' && i+1 < len(r) && r[i+1] == '<' && (i+2 >= len(r) || r[i+2] != '<'):
			h, next := parseHeredoc(r, i+2)
			if h.word == "" {
				cur.WriteString("<<")
				i += 2
				break
			}
			cur.WriteString("<<" + h.word)
			pending = append(pending, h)
			i = next
		case c == '\n':
			flush(";")
			i++
			for _, h := range pending {
				i = skipHeredocBody(r, i, h)
			}
			pending = nil
		case c == ';':
			flush(";")
			i++
		case c == '&' && i+1 < len(r) && r[i+1] == '&':
			flush("&&")
			i += 2
		case c == '|' && i+1 < len(r) && r[i+1] == '|':
			flush("||")
			i += 2
		case c == '|':
			flush("|")
			i++
		default:
			cur.WriteRune(c)
			i++
		}
	}
	flush("")
	if n := len(segs); n > 0 {
		segs[n-1].Sep = ""
	}
	return segs
}

func spaceNL(c rune) rune {
	if c == '\n' {
		return ' '
	}
	return c
}

// parseHeredoc reads the delimiter after `<<` starting at i: an optional `-`,
// then a bare word or a quoted one. It returns the word and the next index.
func parseHeredoc(r []rune, i int) (heredoc, int) {
	var h heredoc
	if i < len(r) && r[i] == '-' {
		h.strip = true
		i++
	}
	for i < len(r) && (r[i] == ' ' || r[i] == '\t') {
		i++
	}
	var word strings.Builder
	if i < len(r) && (r[i] == '\'' || r[i] == '"') {
		q := r[i]
		for i++; i < len(r) && r[i] != q && r[i] != '\n'; i++ {
			word.WriteRune(r[i])
		}
		if i < len(r) && r[i] == q {
			i++
		}
	} else {
		for ; i < len(r) && !strings.ContainsRune(" \t\n;|&<>()", r[i]); i++ {
			word.WriteRune(r[i])
		}
	}
	h.word = word.String()
	return h, i
}

// skipHeredocBody consumes lines from i through the one that closes h, or to
// the end of input when it never closes.
func skipHeredocBody(r []rune, i int, h heredoc) int {
	for i < len(r) {
		end := i
		for end < len(r) && r[end] != '\n' {
			end++
		}
		line := string(r[i:end])
		if h.strip {
			line = strings.TrimSpace(line)
		} else {
			line = strings.TrimRight(line, "\r")
		}
		i = end
		if i < len(r) {
			i++
		}
		if line == h.word {
			return i
		}
	}
	return i
}

// joinSegments writes segments back as one line, newlines as " ; ".
func joinSegments(segs []shellSegment) string {
	var b strings.Builder
	for _, s := range segs {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s.Text)
		if s.Sep != "" {
			b.WriteString(" " + s.Sep)
		}
	}
	return b.String()
}
