package builtin

import (
	"context"
	"strings"
	"testing"
)

func TestBashNonzeroExitAndTimeoutAreToolFailures(t *testing.T) {
	for _, tc := range []struct {
		name, command, contains string
		timeout                 int
		failed                  bool
	}{
		{"success", "printf 'all tests passed'", "all tests passed", 1000, false},
		{"test failure", "printf 'FAILED (failures=1)'; exit 1", "FAILED (failures=1)", 1000, true},
		{"timeout", "sleep 5", "command timed out", 20, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := runBashCommand(context.Background(), tc.command, tc.timeout, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError != tc.failed || !strings.Contains(result.Data.(string), tc.contains) {
				t.Fatalf("result = %#v, want failure=%v with %q", result, tc.failed, tc.contains)
			}
		})
	}
}

func TestBashStartFailureIsNotSuccessfulToolOutput(t *testing.T) {
	original := shellPath
	t.Cleanup(func() { shellPath = original })
	shellPath = "/no-such-shell-for-agent-harness-test"
	result, err := runBashCommand(context.Background(), "true", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Data.(string), "error starting command") {
		t.Fatalf("start failure was reported as success: %#v", result)
	}
}
