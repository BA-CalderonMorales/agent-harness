package ttui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// updateGolden regenerates golden files instead of comparing against
// them. Run `go test ./... -update` after an intentional change.
var updateGolden = flag.Bool("update", false, "update ttui golden files")

var (
	timestampPattern  = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?\b`)
	clockPattern      = regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\b`)
	shortClockPattern = regexp.MustCompile(`\b\d{2}:\d{2}\b`)
	durationPattern   = regexp.MustCompile(`\b\d+(\.\d+)?(ms|µs|s|m)\b`)
	uuidPattern       = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	tempPathPattern   = regexp.MustCompile(`(/tmp|/var/folders)/[^\s"']*`)
)

// Normalize makes a rendered frame comparable across runs: ANSI is
// removed, trailing space is dropped, and the values that change every
// run (timestamps, durations, ids, temp paths) are replaced by
// placeholders.
func Normalize(text string) string {
	text = ansi.Strip(text)
	text = timestampPattern.ReplaceAllString(text, "<TIME>")
	text = clockPattern.ReplaceAllString(text, "<TIME>")
	text = shortClockPattern.ReplaceAllString(text, "<TIME>")
	text = durationPattern.ReplaceAllString(text, "<DURATION>")
	text = uuidPattern.ReplaceAllString(text, "<ID>")
	text = tempPathPattern.ReplaceAllString(text, "<PATH>")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// AssertGolden compares a rendered frame with the stored golden file
// for name, under the caller's testdata/golden directory.
//
// With -update the golden file is rewritten instead, which is how an
// intentional rendering change is reviewed: the diff in the pull request
// is the change.
func AssertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = Normalize(got)
	path := filepath.Join("testdata", "golden", name+".golden")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("ttui: cannot create golden directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("ttui: cannot write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ttui: golden %s is missing (%v); regenerate with: go test ./... -update", path, err)
	}
	// A checkout with CRLF endings must still match: the frame is
	// normalized to LF, so the golden is read the same way.
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	if wantText != got {
		actualPath := path + ".actual"
		_ = os.WriteFile(actualPath, []byte(got), 0o644)
		t.Fatalf("ttui: golden %s does not match the rendered frame.\n%s\nactual written to %s", path, firstDifference(wantText, got), actualPath)
	}
}

// firstDifference describes where two normalized frames diverge.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) && i < len(gotLines); i++ {
		if wantLines[i] != gotLines[i] {
			return "first difference at line " + itoa(i+1) + ":\n  want: " + wantLines[i] + "\n  got:  " + gotLines[i]
		}
	}
	if len(wantLines) != len(gotLines) {
		return "line count differs: want " + itoa(len(wantLines)) + ", got " + itoa(len(gotLines))
	}
	return "frames differ"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
