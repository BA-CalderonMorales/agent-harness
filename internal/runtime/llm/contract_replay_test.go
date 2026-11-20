package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

// Provider contract tests replay recorded HTTP responses from
// testdata/contracts/<provider>/ through the real HTTPClient against an
// httptest server. testdata/contracts/README.md records which fixtures
// were captured from a live server and which were constructed.

// contractKey is the API key the replay client sends. Error assertions
// check it never leaks into a surfaced message.
const contractKey = "sk-contract-0123456789abcdef"

// replayedRequest is what the client sent to the fixture server.
type replayedRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// contractServer serves one recorded response for every request and
// remembers the requests it saw.
type contractServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []replayedRequest
}

func (s *contractServer) seen() []replayedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]replayedRequest(nil), s.requests...)
}

// loadContractFixture parses a raw HTTP response fixture: status line,
// headers, blank line, body (the format the capture script writes).
func loadContractFixture(t *testing.T, provider, name string) (int, http.Header, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "contracts", provider, name))
	if err != nil {
		t.Fatalf("read contract fixture %s/%s: %v", provider, name, err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), nil)
	if err != nil {
		t.Fatalf("parse contract fixture %s/%s: %v", provider, name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read contract fixture body %s/%s: %v", provider, name, err)
	}
	return resp.StatusCode, resp.Header, body
}

func newContractServer(t *testing.T, provider, name string) *contractServer {
	t.Helper()
	status, header, body := loadContractFixture(t, provider, name)
	srv := &contractServer{}
	srv.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		srv.mu.Lock()
		srv.requests = append(srv.requests, replayedRequest{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: reqBody})
		srv.mu.Unlock()
		for key, values := range header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// streamContract replays one chat fixture through HTTPClient.Stream. A
// non-nil error is the one Stream returned before any event.
func streamContract(t *testing.T, provider, name string) (sseObservation, *contractServer, error) {
	t.Helper()
	srv := newContractServer(t, provider, name)
	client := NewHTTPClientWithBaseURL(provider, contractKey, srv.URL+"/v1")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := client.Stream(ctx, contractRequest())
	var got sseObservation
	if err == nil {
		got = observeSSEEvents(drainSSEEvents(events))
	}
	// Every case asserts the wire request too: a client that dropped the
	// tools or sent the wrong model must not pass just because the
	// fixture replays regardless.
	assertWireRequests(t, srv)
	return got, srv, err
}

const (
	contractModel   = "contract-model"
	contractMessage = "List the files in the repo."
	contractTool    = "file_read"
)

// contractSchema is the tool parameter schema every replay sends.
func contractSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
		"required":   []any{"path"},
	}
}

func contractRequest() Request {
	return Request{
		Model:        contractModel,
		SystemPrompt: "You are a contract test.",
		Messages: []types.Message{{
			Role:    types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{Text: contractMessage}},
		}},
		Tools: []tools.Tool{{
			Name:        contractTool,
			Description: "Read a file",
			InputSchema: contractSchema,
		}},
	}
}

// assertWireRequests checks every chat request the client sent: model,
// stream:true, the messages, and the tool definitions (name and
// parameters schema) present in the body.
func assertWireRequests(t *testing.T, srv *contractServer) {
	t.Helper()
	for _, req := range srv.seen() {
		if req.Path != "/v1/chat/completions" {
			continue
		}
		var body struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Type     string `json:"type"`
				Function struct {
					Name       string         `json:"name"`
					Parameters map[string]any `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			t.Fatalf("request body is not JSON: %v\n%s", err, req.Body)
		}
		if body.Model != contractModel {
			t.Errorf("wire model = %q, want %q", body.Model, contractModel)
		}
		if !body.Stream {
			t.Error("wire request is missing stream:true")
		}
		if len(body.Messages) != 2 ||
			body.Messages[0].Role != "system" ||
			body.Messages[1].Role != "user" || body.Messages[1].Content != contractMessage {
			t.Errorf("wire messages = %+v, want system then the user message", body.Messages)
		}
		if len(body.Tools) != 1 || body.Tools[0].Type != "function" || body.Tools[0].Function.Name != contractTool {
			t.Fatalf("wire tools = %+v, want one function tool %q", body.Tools, contractTool)
		}
		if !reflect.DeepEqual(body.Tools[0].Function.Parameters, contractSchema()) {
			t.Errorf("wire tool parameters = %#v, want %#v", body.Tools[0].Function.Parameters, contractSchema())
		}
	}
}

// streamExpectation is what a chat fixture must produce.
type streamExpectation struct {
	text  string
	tools []types.LLMToolUseDelta
	stop  *types.LLMMessageStop
	// errContains fragments must all appear in the stream or Stream error.
	errContains []string
}

func assertStreamContract(t *testing.T, got sseObservation, streamErr error, want streamExpectation) {
	t.Helper()
	var failure error
	if streamErr != nil {
		failure = streamErr
	} else if len(got.errors) > 0 {
		failure = got.errors[0]
	}

	if len(want.errContains) > 0 {
		if failure == nil {
			t.Fatalf("want an error containing %q, got none (stops=%#v)", want.errContains, got.stops)
		}
		assertErrorContains(t, failure, want.errContains...)
		if strings.Contains(failure.Error(), contractKey) {
			t.Fatalf("surfaced error leaks the API key: %v", failure)
		}
		if len(got.stops) != 0 {
			t.Fatalf("an error stream must not also report a clean stop: %#v", got.stops)
		}
		return
	}
	if failure != nil {
		t.Fatalf("unexpected error: %v", failure)
	}
	if text := strings.Join(got.text, ""); text != want.text {
		t.Errorf("text = %q, want %q", text, want.text)
	}
	if !reflect.DeepEqual(got.tools, want.tools) {
		t.Errorf("tool calls = %#v, want %#v", got.tools, want.tools)
	}
	if want.stop != nil {
		if len(got.stops) != 1 || !reflect.DeepEqual(got.stops[0], *want.stop) {
			t.Errorf("stops = %#v, want [%#v]", got.stops, *want.stop)
		}
	}
}

// listModelsContract replays a /models fixture through ListModelsDetailed.
func listModelsContract(t *testing.T, provider, name string) ([]ModelInfo, *contractServer) {
	t.Helper()
	srv := newContractServer(t, provider, name)
	client := NewHTTPClientWithBaseURL(provider, contractKey, srv.URL+"/v1")
	infos, err := client.ListModelsDetailed()
	if err != nil {
		t.Fatalf("ListModelsDetailed() error = %v", err)
	}
	return infos, srv
}

func contextLengths(infos []ModelInfo) map[string]int {
	out := make(map[string]int, len(infos))
	for _, info := range infos {
		out[info.ID] = info.ContextLength
	}
	return out
}
