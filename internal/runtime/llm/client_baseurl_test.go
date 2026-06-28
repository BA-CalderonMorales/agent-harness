package llm

import "testing"

func TestNewHTTPClient_LocalDefaultEndpoint(t *testing.T) {
	client := NewHTTPClient("local", "local")
	if client.BaseURL != "http://127.0.0.1:8080/v1" {
		t.Fatalf("BaseURL = %q, want local llama.cpp endpoint", client.BaseURL)
	}
}

func TestNewHTTPClientWithBaseURL_UsesOverrideAndTrimsSlash(t *testing.T) {
	client := NewHTTPClientWithBaseURL("local", "local", "http://127.0.0.1:9000/v1/")
	if client.BaseURL != "http://127.0.0.1:9000/v1" {
		t.Fatalf("BaseURL = %q, want trimmed override", client.BaseURL)
	}
}
