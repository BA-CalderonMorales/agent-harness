package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

func TestResolveLocalPathHandlesRelativeEnvAndHome(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MODEL_NAME", "ornith.gguf")

	got, err := resolveLocalPath(root, "./models/$MODEL_NAME")
	if err != nil {
		t.Fatalf("resolveLocalPath relative error = %v", err)
	}
	want := filepath.Join(root, "models", "ornith.gguf")
	if got != want {
		t.Fatalf("relative path = %q, want %q", got, want)
	}

	got, err = resolveLocalPath(root, "~/models/ornith.gguf")
	if err != nil {
		t.Fatalf("resolveLocalPath home error = %v", err)
	}
	want = filepath.Join(home, "models", "ornith.gguf")
	if got != want {
		t.Fatalf("home path = %q, want %q", got, want)
	}
}

func TestCheckModelPathReportsExistingAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, "models", "ornith.gguf")
	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("model"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ok := checkModelPath(root, "./models/ornith.gguf")
	if !ok.OK || ok.Warning {
		t.Fatalf("existing model check = %#v, want OK", ok)
	}

	missing := checkModelPath(root, "./models/missing.gguf")
	if !missing.Warning || missing.OK {
		t.Fatalf("missing model check = %#v, want warning", missing)
	}
}

func TestCheckOpenAIEndpointUsesModelsRoute(t *testing.T) {
	var gotPath string
	originalClient := diagnoseHTTPClient
	diagnoseHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() {
		diagnoseHTTPClient = originalClient
	})

	check := checkOpenAIEndpoint(context.Background(), "http://example.test/v1")
	if !check.OK || check.Warning {
		t.Fatalf("endpoint check = %#v, want OK", check)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("probe path = %q, want /v1/models", gotPath)
	}
}

func TestLocalRuntimeChecksUsesProviderSpecificChecks(t *testing.T) {
	root := t.TempDir()
	cfg := &config.LayeredConfig{
		Provider:    "local",
		ModelPath:   "./missing.gguf",
		EndpointURL: "http://127.0.0.1:1/v1",
	}

	checks := localRuntimeChecks(context.Background(), root, cfg)
	if len(checks) != 2 {
		t.Fatalf("local checks len = %d, want 2", len(checks))
	}
	if checks[0].Name != "Model file" || checks[1].Name != "Endpoint" {
		t.Fatalf("checks = %#v", checks)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
