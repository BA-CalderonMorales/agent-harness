package record

import (
	"math/rand"
	"strings"
	"testing"
)

func verifiedBy(cmd string) bool {
	r := Fold(bashCall("c1", cmd, "ok", false))
	return len(r.Verified) > 0
}

func TestQuotesAndHeredocsHideFakeChecks(t *testing.T) {
	no := []string{
		"printf '%s\\n' 'note; go test ./...; end'",
		`echo "run go test ./... later"`,
		"cat <<'EOF'\ngo test ./...\nEOF",
		"cat <<-EOF\n\tmake verify\n\tEOF",
		`grep -r "go vet" docs`,
		"echo 'a\ngo test ./...\nb'",
		"cat <<EOF\ngo test ./...\n", // unterminated heredoc swallows the rest
		"echo 'unbalanced; go test ./...",
		`echo "unbalanced; go test ./...`,
	}
	for _, c := range no {
		if verifiedBy(c) {
			t.Errorf("%q must not count as verified", c)
		}
	}
}

func TestRealChecksAreStillFound(t *testing.T) {
	yes := []string{
		"echo hi; make check",
		"cd project\ngo test ./...",
		"go test ./... && echo done",
		"FOO=1 go test ./...",
		"cat <<EOF\nx\nEOF\ngo test ./...",
		"cat <<-EOF\n\tx\n\tEOF\nmake verify",
		"cd project && \\\ngo test ./...",
		"go \\\ntest ./...",
		"echo 'a\nb' && go vet ./...",
	}
	for _, c := range yes {
		if !verifiedBy(c) {
			t.Errorf("%q should count as verified", c)
		}
	}
}

func TestJoinCommandLinesKeepsStructure(t *testing.T) {
	cases := map[string]string{
		"cd project\ngo test ./...": "cd project ; go test ./...",
		"a && b":                    "a && b",
		"echo 'x\ny'":               "echo 'x y'",
		"cat <<EOF\nbody\nEOF\nls":  "cat <<EOF ; ls",
		"go \\\ntest":               "go  test",
		"printf 'a; b' ; ls":        "printf 'a; b' ; ls",
	}
	for in, want := range cases {
		if got := joinCommandLines(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestShellScannerRobustness(t *testing.T) {
	big := strings.Repeat("a'b\"c<<E\n;|&&\\", 700)
	for _, in := range []string{big, strings.Repeat("'", 10000), strings.Repeat("<<", 5000), strings.Repeat("\\", 10001), strings.Repeat("x\n", 5000) + "<<-"} {
		_ = splitShell(in)
		_ = isCheckCommand(in)
	}
	alphabet := []rune("ab '\"\\\n;|&<-EOFgo test$")
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 4000; i++ {
		n := rng.Intn(60)
		in := make([]rune, n)
		for j := range in {
			in[j] = alphabet[rng.Intn(len(alphabet))]
		}
		s := string(in)
		a, b := joinCommandLines(s), joinCommandLines(s)
		checkA, checkB := isCheckCommand(s), isCheckCommand(s)
		if a != b || checkA != checkB {
			t.Fatalf("nondeterministic on %q", s)
		}
	}
	for i := 0; i < 500; i++ {
		raw := make([]byte, rng.Intn(80))
		rng.Read(raw)
		_ = joinCommandLines(string(raw))
	}
}
