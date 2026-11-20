package llm

import (
	"bufio"
	"bytes"
	"context"
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
		_, _ = io.Copy(io.Discard, r.Body)
		srv.mu.Lock()
		srv.requests = append(srv.requests, replayedRequest{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()})
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
	events, err := client.Stream(ctx, Request{
		Model:    "contract-model",
		Messages: []types.Message{},
	})
	if err != nil {
		return sseObservation{}, srv, err
	}
	return observeSSEEvents(drainSSEEvents(events)), srv, nil
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
