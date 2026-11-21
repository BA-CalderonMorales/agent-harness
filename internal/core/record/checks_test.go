package record

import (
	"fmt"
	"reflect"
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
