package tui

import (
	"strings"
	"testing"
)

// TestReadinessNoticeCarriesItsFix pins the boot precheck's usefulness. The
// probe already runs before the first prompt; the point of the notice is
// that a credential failure reads as a direction — where to get a key and
// which key opens the wizard — rather than as a dead end the user only
// discovers by watching a turn fail.
func TestReadinessNoticeCarriesItsFix(t *testing.T) {
	notice := readinessNotice("anthropic", 4, "invalid API key")
	if !strings.Contains(notice, "Provider misconfigured") {
		t.Fatalf("notice lost the verdict: %q", notice)
	}
	if !strings.Contains(notice, "invalid API key") {
		t.Fatalf("notice lost the provider's own message: %q", notice)
	}
	steps, ok := keyStepsFor("anthropic")
	if !ok {
		t.Fatal("anthropic lost its key journey")
	}
	if !strings.Contains(notice, steps.URL) {
		t.Fatalf("notice does not say where to get a key (%s): %q", steps.URL, notice)
	}
	if !strings.Contains(notice, "press l") {
		t.Fatalf("notice does not say how to fix it: %q", notice)
	}
}

// TestReadinessNoticeStaysQuietForLocalRuntimes pins that a local runtime
// gets no signup pointer: there is nothing to sign up for, and a "get a
// key" line for ollama is noise that trains the reader to ignore notices.
func TestReadinessNoticeStaysQuietForLocalRuntimes(t *testing.T) {
	for _, local := range []string{"local", "ollama", "flm"} {
		if line := keyRemedyLine(local); line != "" {
			t.Fatalf("%s advertised a key remedy: %q", local, line)
		}
		notice := readinessNotice(local, 4, "endpoint not found")
		if strings.Contains(notice, "Get a key") {
			t.Fatalf("%s notice advertised a key: %q", local, notice)
		}
	}
}

// TestReadinessNoticeWordingIsShared pins the single home for the verdict
// sentences: the transcript notice and the Logs line are composed from one
// function, so they cannot drift apart.
func TestReadinessNoticeWordingIsShared(t *testing.T) {
	cases := []struct {
		readiness int
		want      string
	}{
		{1, "Provider ready: "},
		{2, "Provider warning: "},
		{3, "Provider unavailable: "},
		{4, "Provider misconfigured: "},
	}
	for _, c := range cases {
		notice := readinessNotice("openai", c.readiness, "detail")
		if !strings.HasPrefix(notice, c.want) {
			t.Fatalf("readiness %d: notice %q does not start with %q", c.readiness, notice, c.want)
		}
	}
}
