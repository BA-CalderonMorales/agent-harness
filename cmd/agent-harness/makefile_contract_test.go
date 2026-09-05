package main

import (
	"os"
	"strings"
	"testing"
)

// The make contract: `make run` launches the artifact `make build`
// produced. The old shape — `go run ./cmd/agent-harness` — recompiled
// and re-linked the whole binary on every launch, and the linker's
// thread spawn died on a saturated box (fatal error: newosproc,
// errno=11, "may need to increase max user processes") before the app
// ever started. One build, one link, then exec: the crash surface is
// the redundant link, and the artifact that gets verified is the
// artifact that runs.

func readMakefile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	return string(data)
}

// targetLine returns the `target:` declaration line.
func targetLine(makefile, target string) string {
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, target+":") {
			return line
		}
	}
	return ""
}

// recipe returns the indented recipe block of a target: every line
// after the declaration until the next unindented line.
func recipe(makefile, target string) string {
	var block []string
	inRecipe := false
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, target+":") {
			inRecipe = true
			continue
		}
		if !inRecipe {
			continue
		}
		if line != "" && !strings.HasPrefix(line, "\t") {
			break // the next target's declaration
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n")
}

// TestMakeRunExecsBuiltBinary is the run-path contract, red/green:
// no `go run` (the redundant recompile+relink), the built artifact as
// the exec target, and a build dependency so a bare `make run` on a
// clean tree still launches.
func TestMakeRunExecsBuiltBinary(t *testing.T) {
	mk := readMakefile(t)

	if targetLine(mk, "run") == "" {
		t.Fatal("Makefile has no run target")
	}
	run := recipe(mk, "run")
	if run == "" {
		t.Fatal("run target has an empty recipe")
	}

	if strings.Contains(run, "go run") {
		t.Fatalf("make run invokes `go run` — the redundant recompile+relink is the thread-exhaustion crash surface:\n%s", run)
	}
	if !strings.Contains(run, "$(BUILD_DIR)/$(BINARY_NAME)") {
		t.Fatalf("make run must exec the built artifact $(BUILD_DIR)/$(BINARY_NAME):\n%s", run)
	}
	if !strings.Contains(targetLine(mk, "run"), "build") {
		t.Fatalf("make run must depend on build so a bare `make run` on a clean tree still launches:\n%s", targetLine(mk, "run"))
	}
}
