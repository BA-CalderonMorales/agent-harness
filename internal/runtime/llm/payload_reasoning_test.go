package llm

import (
	"encoding/json"
	"testing"
)

// decodePayload builds and unmarshals a payload for the given provider.
func decodePayload(t *testing.T, provider string, effort string) map[string]any {
	t.Helper()
	client := NewHTTPClientWithBaseURL(provider, "test-key", "https://example.test/v1")
	payload, err := client.buildPayload(Request{
		Model:           "test-model",
		ReasoningEffort: effort,
	})
	if err != nil {
		t.Fatalf("buildPayload(%s): %v", provider, err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal(%s): %v", provider, err)
	}
	return result
}

// TestNvidiaPayloadUsesTopLevelThinkingParams pins the live bug fix:
// NVIDIA's hosted API (additionalProperties: false) rejects a raw
// extra_body key with 400 "Unsupported parameter(s)"; the thinking
// params must be top-level, exactly as NVIDIA's own docs show.
func TestNvidiaPayloadUsesTopLevelThinkingParams(t *testing.T) {
	result := decodePayload(t, "nvidia", "medium")

	if _, ok := result["extra_body"]; ok {
		t.Fatalf("nvidia payload must not carry a raw extra_body key: %v", result)
	}
	if _, ok := result["reasoning_effort"]; ok {
		t.Fatalf("nvidia payload must not carry reasoning_effort: %v", result)
	}
	budget, ok := result["reasoning_budget"].(float64)
	if !ok || budget != 4096 {
		t.Fatalf("reasoning_budget = %v, want 4096 for medium effort: %v", result["reasoning_budget"], result)
	}
	kwargs, ok := result["chat_template_kwargs"].(map[string]any)
	if !ok || kwargs["enable_thinking"] != true {
		t.Fatalf("chat_template_kwargs.enable_thinking must be true: %v", result)
	}
}

func TestNvidiaPayloadSkipsThinkingWhenEffortOff(t *testing.T) {
	result := decodePayload(t, "nvidia", "off")

	if _, ok := result["reasoning_budget"]; ok {
		t.Fatalf("effort off must not set reasoning_budget: %v", result)
	}
	if _, ok := result["chat_template_kwargs"]; ok {
		t.Fatalf("effort off must not set chat_template_kwargs: %v", result)
	}
}

func TestNvidiaPayloadBudgetMapping(t *testing.T) {
	for effort, want := range map[string]float64{"low": 1024, "medium": 4096, "high": 16384} {
		result := decodePayload(t, "nvidia", effort)
		if got := result["reasoning_budget"]; got != want {
			t.Fatalf("nvidia reasoning_budget for %q = %v, want %v", effort, got, want)
		}
	}
}

// TestReasoningParamsProviderMatrix is the rhyme: every provider carries
// exactly the reasoning mechanism its API documents, and nothing else.
func TestReasoningParamsProviderMatrix(t *testing.T) {
	// Provider -> the keys its payload must carry (or must not).
	cases := []struct {
		provider   string
		wantKeys   []string
		absentKeys []string
	}{
		{"openai", []string{"reasoning_effort"}, []string{"thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"openrouter", []string{"reasoning_effort"}, []string{"thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"nvidia", []string{"reasoning_budget", "chat_template_kwargs"}, []string{"reasoning_effort", "thinking"}},
		{"anthropic", []string{"thinking"}, []string{"reasoning_effort", "reasoning_budget"}},
		// Undocumented knobs: local gateways, ollama, and fireworks get
		// no reasoning parameter at all - an undocumented parameter is
		// how the nvidia extra_body 400 shipped.
		{"local", nil, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"ollama", nil, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"fireworks", nil, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
	}

	for _, tc := range cases {
		result := decodePayload(t, tc.provider, "medium")
		for _, key := range tc.wantKeys {
			if _, ok := result[key]; !ok {
				t.Fatalf("%s payload missing %q: %v", tc.provider, key, result)
			}
		}
		for _, key := range tc.absentKeys {
			if _, ok := result[key]; ok {
				t.Fatalf("%s payload must not carry %q: %v", tc.provider, key, result)
			}
		}
	}
}

// TestAnthropicPayloadThinkingBudget: anthropic expresses effort via the
// extended-thinking budget, never via OpenAI reasoning_effort.
func TestAnthropicPayloadThinkingBudget(t *testing.T) {
	result := decodePayload(t, "anthropic", "medium")
	thinking, ok := result["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("anthropic payload missing thinking block: %v", result)
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("thinking type = %v, want enabled", thinking["type"])
	}
	if budget := thinking["budget_tokens"]; budget != float64(4096) {
		t.Fatalf("budget_tokens = %v, want 4096 for medium effort", budget)
	}
}
