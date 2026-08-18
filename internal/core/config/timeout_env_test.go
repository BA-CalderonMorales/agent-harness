package config

import (
	"testing"
	"time"
)

// TestLayeredLoader_TimeoutDefaultsScaleToProvider verifies that the
// stream-idle watchdog and HTTP client timeout default to local-model
// friendly windows for local/ollama providers (CPU prompt eval takes
// minutes) while keeping tight guards for hosted providers.
func TestLayeredLoader_TimeoutDefaultsScaleToProvider(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	cases := []struct {
		provider        string
		wantIdle        time.Duration
		wantHTTP        time.Duration
	}{
		{"local", 30 * time.Minute, 45 * time.Minute},
		{"ollama", 30 * time.Minute, 45 * time.Minute},
		{"openai", 90 * time.Second, 120 * time.Second},
		{"openrouter", 90 * time.Second, 120 * time.Second},
		{"anthropic", 90 * time.Second, 120 * time.Second},
	}
	for _, tc := range cases {
		t.Setenv("AH_PROVIDER", tc.provider)
		cfg, err := NewLayeredLoader(t.TempDir()).Load()
		if err != nil {
			t.Fatalf("provider %s: Load() error = %v", tc.provider, err)
		}
		if cfg.StreamIdleTimeout != tc.wantIdle {
			t.Errorf("provider %s: StreamIdleTimeout = %v, want %v", tc.provider, cfg.StreamIdleTimeout, tc.wantIdle)
		}
		if cfg.HTTPTimeout != tc.wantHTTP {
			t.Errorf("provider %s: HTTPTimeout = %v, want %v", tc.provider, cfg.HTTPTimeout, tc.wantHTTP)
		}
	}
}

// TestLayeredLoader_TimeoutEnvOverrides verifies AH_STREAM_IDLE_TIMEOUT and
// AH_HTTP_TIMEOUT (and their AGENT_HARNESS_* aliases) override the
// provider defaults, accepting Go durations.
func TestLayeredLoader_TimeoutEnvOverrides(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	t.Setenv("AH_PROVIDER", "openai")
	t.Setenv("AH_STREAM_IDLE_TIMEOUT", "5m")
	t.Setenv("AGENT_HARNESS_HTTP_TIMEOUT", "7m")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StreamIdleTimeout != 5*time.Minute {
		t.Errorf("StreamIdleTimeout = %v, want 5m", cfg.StreamIdleTimeout)
	}
	if cfg.HTTPTimeout != 7*time.Minute {
		t.Errorf("HTTPTimeout = %v, want 7m", cfg.HTTPTimeout)
	}
	if !cfg.TimeoutPinned {
		t.Error("TimeoutPinned = false, want true when env overrides are set")
	}
}

// TestLayeredLoader_TimeoutEnvIntegerSeconds verifies plain-integer values
// are accepted as seconds.
func TestLayeredLoader_TimeoutEnvIntegerSeconds(t *testing.T) {
	t.Setenv("AGENT_HARNESS_CONFIG_HOME", t.TempDir())
	clearConfigEnv(t)

	t.Setenv("AH_STREAM_IDLE_TIMEOUT", "600")

	cfg, err := NewLayeredLoader(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.StreamIdleTimeout != 10*time.Minute {
		t.Errorf("StreamIdleTimeout = %v, want 10m", cfg.StreamIdleTimeout)
	}
}
