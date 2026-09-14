package config

import "testing"

func TestDropNoOpValues(t *testing.T) {
	data := map[string]interface{}{
		"context_length":   0,
		"max_tokens":       0,
		"endpoint_url":     "",
		"model":            "",
		"provider":         "openai",
		"tagline":          "",
		"perm_read":        false,
		"temperature":      0.0,
		"reasoning_effort": "high",
	}

	dropNoOpValues(data)

	for _, gone := range []string{"context_length", "max_tokens", "endpoint_url", "model"} {
		if _, ok := data[gone]; ok {
			t.Errorf("%s should have been dropped as a no-op value", gone)
		}
	}
	for _, kept := range []string{"provider", "tagline", "perm_read", "temperature", "reasoning_effort"} {
		if _, ok := data[kept]; !ok {
			t.Errorf("%s must survive normalization", kept)
		}
	}
}
