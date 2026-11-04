package llm

import (
	"encoding/json"
	"testing"
)

// decodePayload builds and unmarshals a payload for the given provider.
func decodePayload(t *testing.T, provider string, effort string) map[string]any {
	return decodePayloadWithModel(t, provider, effort, "test-model")
}

func decodePayloadWithModel(t *testing.T, provider string, effort string, model string) map[string]any {
	t.Helper()
	client := NewHTTPClientWithBaseURL(provider, "test-key", "https://example.test/v1")
	payload, err := client.buildPayload(Request{
		Model:           model,
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
	result := decodePayloadWithModel(t, "nvidia", "medium", "nvidia/nemotron-3-super")

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
		result := decodePayloadWithModel(t, "nvidia", effort, "nvidia/nemotron-3-super")
		if got := result["reasoning_budget"]; got != want {
			t.Fatalf("nvidia reasoning_budget for %q = %v, want %v", effort, got, want)
		}
	}
}

// Non-Nemotron models on NVIDIA validate strictly: reasoning_budget is
// absent (the deepseek 400), while the template knob stays.
func TestNvidiaPayloadSkipsBudgetForNonNemotronModels(t *testing.T) {
	for _, model := range []string{"deepseek-ai/deepseek-v4-flash-0731", "meta/llama-3.3-70b-instruct", "qwen/qwen2.5-coder-32b"} {
		result := decodePayloadWithModel(t, "nvidia", "medium", model)
		if _, ok := result["reasoning_budget"]; ok {
			t.Fatalf("%s: reasoning_budget must be absent (strict validation 400s on it): %v", model, result)
		}
		if kwargs, ok := result["chat_template_kwargs"].(map[string]any); !ok || kwargs["enable_thinking"] != true {
			t.Fatalf("%s: chat_template_kwargs.enable_thinking must stay: %v", model, result)
		}
	}
}

// TestReasoningParamsProviderMatrix is the rhyme: every provider carries
// exactly the reasoning mechanism its API documents, and nothing else.
func TestReasoningParamsProviderMatrix(t *testing.T) {
	// Provider -> the keys its payload must carry (or must not).
	cases := []struct {
		provider   string
		model      string
		wantKeys   []string
		absentKeys []string
	}{
		{"openai", "test-model", []string{"reasoning_effort"}, []string{"thinking", "reasoning_budget", "chat_template_kwargs", "reasoning"}},
		{"openrouter", "test-model", []string{"reasoning"}, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"nvidia", "nvidia/nemotron-3-super", []string{"reasoning_budget", "chat_template_kwargs"}, []string{"reasoning_effort", "thinking"}},
		{"nvidia", "deepseek-ai/deepseek-v4-flash", []string{"chat_template_kwargs"}, []string{"reasoning_effort", "reasoning_budget"}},
		{"anthropic", "test-model", []string{"thinking"}, []string{"reasoning_effort", "reasoning_budget"}},
		// Undocumented knobs: local gateways, ollama, and fireworks get
		// no reasoning parameter at all - an undocumented parameter is
		// how the nvidia extra_body 400 shipped.
		{"local", "test-model", []string{}, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"ollama", "test-model", []string{}, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
		{"fireworks", "test-model", []string{}, []string{"reasoning_effort", "thinking", "reasoning_budget", "chat_template_kwargs"}},
	}

	for _, tc := range cases {
		result := decodePayloadWithModel(t, tc.provider, "medium", tc.model)
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

// TestOpenRouterPayloadUsesReasoningObject pins the hosted latency fix:
// OpenRouter exposes effort as reasoning.effort, and reasoning_effort is
// not in a free model's supported_parameters, so the old field was dropped
// and the model kept its default (long) reasoning budget.
func TestOpenRouterPayloadUsesReasoningObject(t *testing.T) {
	result := decodePayloadWithModel(t, "openrouter", "low", "nvidia/nemotron-3.5-lightning:free")

	reasoning, ok := result["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("openrouter payload missing reasoning object: %v", result)
	}
	if reasoning["effort"] != "low" {
		t.Fatalf("reasoning.effort = %v, want low: %v", reasoning["effort"], result)
	}
	if _, ok := result["reasoning_effort"]; ok {
		t.Fatalf("openrouter payload must not carry reasoning_effort: %v", result)
	}
}

// TestTemperatureZeroIsSentWhenChosen pins the trust bug: 0.0 is a
// legal deterministic setting, but the old "only send > 0" rule made it
// silently fall back to the provider default (often 1.0) - the exact
// condition behind unstable free-model prose.
func TestTemperatureZeroIsSentWhenChosen(t *testing.T) {
	client := NewHTTPClientWithBaseURL("openrouter", "test-key", "https://example.test/v1")

	payload, err := client.buildPayload(Request{Model: "test-model", Temperature: 0, TemperatureSet: true})
	if err != nil {
		t.Fatal(err)
	}
	var chosen map[string]any
	if err := json.Unmarshal(payload, &chosen); err != nil {
		t.Fatal(err)
	}
	if got, ok := chosen["temperature"]; !ok || got != float64(0) {
		t.Fatalf("explicit temperature 0 was dropped: %v", chosen)
	}

	payload, err = client.buildPayload(Request{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	var unset map[string]any
	if err := json.Unmarshal(payload, &unset); err != nil {
		t.Fatal(err)
	}
	if _, ok := unset["temperature"]; ok {
		t.Fatalf("unset temperature must stay absent: %v", unset)
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
