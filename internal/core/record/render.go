package record

import (
	"fmt"
	"strings"
)

// Header opens every rendered record.
const Header = "[Session record]"

// charsPerToken matches the repo's estimate (pkg/messages.EstimateTokens:
// four characters per token, rounded up).
const charsPerToken = 4

// Render writes the record as compact plain text in a fixed order, never
// longer than maxTokens by the repo's 4-chars-per-token estimate. When the
// record is too big it sheds the oldest items first, section by section in a
// fixed order, then shortens the goal. An empty record renders as "".
func Render(r Record, maxTokens int) string {
	if maxTokens <= 0 || isBlank(r) {
		return ""
	}
	maxChars := maxTokens * charsPerToken
	r = clone(r)

	// Least valuable first. Each section loses its oldest item before the
	// next section is touched.
	sheds := []func(*Record) bool{
		func(r *Record) bool { return dropFirst(&r.Cmds) },
		func(r *Record) bool { return dropFirst(&r.Files) },
		func(r *Record) bool { return dropFirst(&r.Verified) },
		func(r *Record) bool { return dropFirst(&r.Denied) },
		func(r *Record) bool { return dropFirst(&r.Constraints) },
		func(r *Record) bool { return dropFirst(&r.Todos) },
		func(r *Record) bool { return dropFirst(&r.Pending) },
	}
	out := body(r)
	for _, shed := range sheds {
		for chars(out) > maxChars && shed(&r) {
			out = body(r)
		}
	}
	for chars(out) > maxChars && len([]rune(r.Goal)) > 1 {
		r.Goal = clip(r.Goal, max(1, len([]rune(r.Goal))-8))
		out = body(r)
	}
	if chars(out) > maxChars {
		// Bytes are what the estimate counts; cut on a rune boundary.
		cut := []rune(out)
		for len(string(cut)) > maxChars {
			cut = cut[:len(cut)-1]
		}
		out = string(cut)
	}
	return out
}

func dropFirst[T any](list *[]T) bool {
	if len(*list) == 0 {
		return false
	}
	*list = (*list)[1:]
	return true
}

// chars counts bytes, as pkg/messages.EstimateTokens does.
func chars(s string) int { return len(s) }

func isBlank(r Record) bool {
	return r.Goal == "" && len(r.Constraints) == 0 && len(r.Files) == 0 && len(r.Cmds) == 0 &&
		len(r.Todos) == 0 && len(r.Verified) == 0 && len(r.Pending) == 0 && len(r.Denied) == 0
}

func body(r Record) string {
	var b strings.Builder
	b.WriteString(Header)
	if r.Goal != "" {
		fmt.Fprintf(&b, "\nGoal: %s", r.Goal)
	}
	list(&b, "Constraints", r.Constraints, func(c string) string { return c })
	list(&b, "Files", r.Files, func(f FileTouch) string {
		if f.N > 1 {
			return fmt.Sprintf("%s %s x%d", f.Op, f.Path, f.N)
		}
		return f.Op + " " + f.Path
	})
	list(&b, "Commands", r.Cmds, func(c CmdRun) string {
		if c.Failed {
			if c.Err != "" {
				return fmt.Sprintf("FAILED %s -> %s", c.Cmd, c.Err)
			}
			return "FAILED " + c.Cmd
		}
		return "ok " + c.Cmd
	})
	list(&b, "Todos", r.Todos, func(t Todo) string { return "[" + t.Status + "] " + t.Text })
	list(&b, "Verified", r.Verified, func(v string) string { return v })
	list(&b, "Pending", r.Pending, func(p string) string { return p })
	list(&b, "Denied", r.Denied, func(d string) string { return d })
	return b.String()
}

func list[T any](b *strings.Builder, title string, items []T, line func(T) string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:", title)
	for _, item := range items {
		b.WriteString("\n- ")
		b.WriteString(line(item))
	}
}
