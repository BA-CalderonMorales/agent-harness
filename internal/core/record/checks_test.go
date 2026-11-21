package record

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestIsCheckCommand(t *testing.T) {
	yes := []string{
		"go test ./...", "go vet ./...", "go build ./cmd/x", "cd x && go test ./...", "make verify", "make lint",
		"npm test", "npm run build", "pnpm run lint", "yarn test", "pytest -q", "cargo clippy", "cargo test --all",
		"golangci-lint run", "staticcheck ./...", "gofmt -l .", "tsc --noEmit", "mvn test", "gradle build",
		"FOO=1 go test ./...", "echo hi; make check", "go vet ./... | tee out",
	}
	no := []string{
		"cat build.log", "git diff -- test/foo.go", "ls test", "grep lint README.md", "echo go test",
		"gofmt main.go", "make", "make clean", "go run ./cmd", "npm install", "cat go test.txt", "",
	}
	for _, c := range yes {
		if !isCheckCommand(c) {
			t.Errorf("%q should be a check", c)
		}
	}
	for _, c := range no {
		if isCheckCommand(c) {
			t.Errorf("%q should not be a check", c)
		}
	}
}

func TestVerifiedIgnoresNonCheckCommands(t *testing.T) {
	r := fold(bashCall("c0", "go test ./...", "ok", false), bashCall("c1", "make verify", "ok", false))
	for i := 0; i < 50; i++ {
		r = Apply(r, use(fmt.Sprint("n", i), "bash", map[string]any{"command": fmt.Sprintf("cat build%d.log", i)}))
		r = Apply(r, result(fmt.Sprint("n", i), "x", false))
	}
	if want := []string{"go test ./...", "make verify"}; !reflect.DeepEqual(r.Verified, want) {
		t.Fatalf("verified = %v", r.Verified)
	}
}

// A multiline shell command keeps its command boundaries through the full
// Apply path: storing the call must not fuse "cd project" and "go test"
// into one segment.
func TestVerifiedSeesChecksInMultilineCommands(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{"cd then check on the next line", "cd project\ngo test ./...", true},
		{"CRLF line endings", "cd project\r\ngo vet ./...", true},
		{"blank lines and indentation", "cd project\n\n   make verify\n", true},
		{"check on the first line", "go test ./...\necho done", true},
		{"backslash continuation of one command", "go test \\\n  ./...", true},
		{"no check on any line", "cd project\ncat build.log", false},
		{"check words only as arguments across lines", "git diff \\\n  -- test/foo.go", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fold(bashCall("m1", c.cmd, "ok", false))
			if got := len(r.Verified) == 1; got != c.want {
				t.Fatalf("verified = %v, want a check recorded: %v", r.Verified, c.want)
			}
			if len(r.Cmds) != 1 || strings.Contains(r.Cmds[0].Cmd, "\n") {
				t.Fatalf("recorded command must be a single line: %#v", r.Cmds)
			}
		})
	}
}
