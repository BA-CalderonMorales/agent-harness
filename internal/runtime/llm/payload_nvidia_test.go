package llm

import (
	"encoding/json"
	"testing"
)

func TestBuildPayloadNvidiaUsesThinkingBudgetNotReasoningEffort(t *testing.T) {
	client := NewHTTPClientWithBaseURL("nvidia", "nvapi-test", "https://integrate.api.nvidia.com/v1")
	req := Request{
		Model:           "nvidia/nemotron-3.5-lightning-30b-a3b",
		ReasoningEffort: "medium",
	}
	payload, err := client.buildPayload(req)
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["reasoning_effort"]; ok {
		t.Fatalf("nvidia payload must not carry reasoning_effort: %s", payload)
	}
	extra, ok := result["extra_body"].(map[string]any)
	if !ok {
		t.Fatalf("nvidia payload must carry extra_body: %s", payload)
	}
	kwargs, ok := extra["chat_template_kwargs"].(map[string]any)
	if !ok || kwargs["enable_thinking"] != true {
		t.Fatalf("chat_template_kwargs.enable_thinking must be true: %s", payload)
	}
	if budget, ok := extra["reasoning_budget"].(float64); !ok || budget != 4096 {
		t.Fatalf("reasoning_budget = %v, want 4096 for medium effort: %s", extra["reasoning_budget"], payload)
	}
}

func TestBuildPayloadNvidiaSkipsThinkingWhenEffortOff(t *testing.T) {
	client := NewHTTPClientWithBaseURL("nvidia", "nvapi-test", "https://integrate.api.nvidia.com/v1")
	req := Request{
		Model:           "nvidia/nemotron-3.5-lightning-30b-a3b",
		ReasoningEffort: "off",
	}
	payload, err := client.buildPayload(req)
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["extra_body"]; ok {
		t.Fatalf("effort off must not enable thinking: %s", payload)
	}
}

func TestBuildPayloadOtherProvidersKeepReasoningEffort(t *testing.T) {
	client := NewHTTPClientWithBaseURL("openrouter", "sk-test", "https://openrouter.ai/api/v1")
	req := Request{
		Model:           "nvidia/nemotron-3-super-120b-a12b:free",
		ReasoningEffort: "high",
	}
	payload, err := client.buildPayload(req)
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, ok := result["reasoning_effort"].(string); !ok || got != "high" {
		t.Fatalf("reasoning_effort = %v, want high: %s", result["reasoning_effort"], payload)
	}
	if _, ok := result["extra_body"]; ok {
		t.Fatalf("openrouter payload must not carry extra_body: %s", payload)
	}
}
