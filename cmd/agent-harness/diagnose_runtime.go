package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/core/config"
)

type runtimeCheck struct {
	Name    string
	OK      bool
	Detail  string
	Warning bool
}

var diagnoseHTTPClient = http.DefaultClient

func localRuntimeChecks(ctx context.Context, cwd string, cfg *config.LayeredConfig) []runtimeCheck {
	if cfg == nil {
		return nil
	}
	switch cfg.Provider {
	case "local":
		return []runtimeCheck{
			checkModelPath(cwd, cfg.ModelPath),
			checkOpenAIEndpoint(ctx, cfg.EndpointURL),
		}
	case "ollama":
		return []runtimeCheck{checkOllamaEndpoint(ctx)}
	case "flm":
		return []runtimeCheck{checkFLMEndpoint(ctx)}
	default:
		return nil
	}
}

func checkModelPath(cwd, path string) runtimeCheck {
	if path == "" {
		return runtimeCheck{Name: "Model file", Warning: true, Detail: "model_path is not configured"}
	}
	resolved, err := resolveLocalPath(cwd, path)
	if err != nil {
		return runtimeCheck{Name: "Model file", Warning: true, Detail: err.Error()}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return runtimeCheck{Name: "Model file", Warning: true, Detail: fmt.Sprintf("%s is missing", resolved)}
	}
	if info.IsDir() {
		return runtimeCheck{Name: "Model file", Warning: true, Detail: fmt.Sprintf("%s is a directory", resolved)}
	}
	return runtimeCheck{Name: "Model file", OK: true, Detail: resolved}
}

func checkOpenAIEndpoint(ctx context.Context, endpoint string) runtimeCheck {
	if endpoint == "" {
		return runtimeCheck{Name: "Endpoint", Warning: true, Detail: "endpoint_url is not configured"}
	}
	return checkHTTPEndpoint(ctx, strings.TrimRight(endpoint, "/")+"/models")
}

func checkOllamaEndpoint(ctx context.Context) runtimeCheck {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	return checkHTTPEndpoint(ctx, strings.TrimRight(host, "/")+"/api/tags")
}

// checkFLMEndpoint probes the FastFlowLM server. The FLM API needs no
// auth, so /v1/models answers without headers.
func checkFLMEndpoint(ctx context.Context) runtimeCheck {
	url := "http://127.0.0.1:52625/v1/models"
	if env := os.Getenv("AH_ENDPOINT_URL"); env != "" {
		url = strings.TrimRight(env, "/") + "/models"
	}
	return checkHTTPEndpoint(ctx, url)
}

func checkHTTPEndpoint(ctx context.Context, url string) runtimeCheck {
	ctx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return runtimeCheck{Name: "Endpoint", Warning: true, Detail: err.Error()}
	}
	resp, err := diagnoseHTTPClient.Do(req)
	if err != nil {
		return runtimeCheck{Name: "Endpoint", Warning: true, Detail: fmt.Sprintf("%s is not reachable: %v", url, err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return runtimeCheck{Name: "Endpoint", Warning: true, Detail: fmt.Sprintf("%s returned HTTP %d", url, resp.StatusCode)}
	}
	return runtimeCheck{Name: "Endpoint", OK: true, Detail: url}
}
