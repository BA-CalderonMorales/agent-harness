package planning

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

type VerificationResult struct {
	Command string
	Passed  bool
	Output  string
	Error   error
}

// RunDeterministicGoTests runs normal Go tests with live provider e2e and API
// key environment variables disabled for this subprocess.
func RunDeterministicGoTests(ctx context.Context, root string) VerificationResult {
	const command = "go test ./..."
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = root
	cmd.Env = deterministicEnv(os.Environ())

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return VerificationResult{
		Command: command,
		Passed:  err == nil,
		Output:  strings.TrimSpace(output.String()),
		Error:   err,
	}
}

func deterministicEnv(env []string) []string {
	blocked := map[string]bool{
		"AH_E2E_OPENROUTER":     true,
		"AH_API_KEY":            true,
		"AGENT_HARNESS_API_KEY": true,
		"OPENROUTER_API_KEY":    true,
	}
	cleaned := make([]string, 0, len(env)+1)
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && blocked[key] {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	return append(cleaned, "AH_E2E_OPENROUTER=0")
}
