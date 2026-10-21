package state

import (
	"strings"
	"testing"
)

// TestRedactExportStringCoversCommonCredentialShapes pins the shapes a
// maintainer is most likely to paste into an exported transcript:
// authorization headers, x-api-key headers, provider token prefixes,
// private keys, secret references, and local paths.
func TestRedactExportStringCoversCommonCredentialShapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		leaks []string
		want  string
	}{
		{
			name:  "bearer header",
			input: "Authorization: Bearer abc.def.ghi",
			leaks: []string{"abc.def.ghi"},
			want:  "<redacted>",
		},
		{
			name:  "lowercase bearer header",
			input: "authorization=bearer s3cr3t-value",
			leaks: []string{"s3cr3t-value"},
			want:  "<redacted>",
		},
		{
			name:  "x-api-key header",
			input: "x-api-key: dummy",
			leaks: []string{"dummy"},
			want:  "<redacted>",
		},
		{
			name:  "xai token",
			input: "key xai-abc123DEF456ghi",
			leaks: []string{"xai-abc123DEF456ghi"},
			want:  "<redacted>",
		},
		{
			name:  "google api key",
			input: "AIzaSyA1234567890abcdefghijklmnopqrst",
			leaks: []string{"AIzaSyA1234567890abcdefghijklmnopqrst"},
			want:  "<redacted>",
		},
		{
			name:  "hugging face token",
			input: "hf_abcdefghijklmnopqrstuvwx",
			leaks: []string{"hf_abcdefghijklmnopqrstuvwx"},
			want:  "<redacted>",
		},
		{
			name:  "gitlab token",
			input: "glpat-abc123DEF456",
			leaks: []string{"glpat-abc123DEF456"},
			want:  "<redacted>",
		},
		{
			name:  "npm token",
			input: "npm_abcdefghijklmnopqrstuvwx",
			leaks: []string{"npm_abcdefghijklmnopqrstuvwx"},
			want:  "<redacted>",
		},
		{
			name:  "private key block",
			input: "-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----",
			leaks: []string{
				"MIIEow",
			},
			want: "<redacted-private-key>",
		},
		{
			name:  "json header object",
			input: `{"Authorization":"Bearer opaque-value","x-api-key":"dummy"}`,
			leaks: []string{"opaque-value", `"dummy"`},
			want:  "Authorization",
		},
		{
			name:  "secret reference",
			input: "loaded via secret://cmd:gcloud secrets versions access latest --secret=openai-key now",
			leaks: []string{"gcloud secrets", "openai-key"},
			want:  "<secret-ref>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactExportString(tt.input)
			for _, leak := range tt.leaks {
				if strings.Contains(got, leak) {
					t.Fatalf("redactExportString(%q) = %q, still contains %q", tt.input, got, leak)
				}
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("redactExportString(%q) = %q, want it to contain %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRedactExportMapTreatsHeaderMapsAsSensitive covers structured
// exports, where a header travels as a map key rather than as visible
// "Header: value" text.
func TestRedactExportMapTreatsHeaderMapsAsSensitive(t *testing.T) {
	got := redactExportMap(map[string]any{
		"Authorization": "Bearer opaque-value",
		"x-api-key":     "dummy",
		"Cookie":        "session=abc123",
		"nested": map[string]any{
			"authorization": "Basic dXNlcjpwYXNz",
		},
		"safe": "keep",
	})
	for _, key := range []string{"Authorization", "x-api-key", "Cookie", "authorization"} {
		if key == "authorization" {
			nested, ok := got["nested"].(map[string]any)
			if !ok || nested[key] != "<redacted>" {
				t.Fatalf("nested[%q] = %#v, want <redacted>", key, nested)
			}
			continue
		}
		if got[key] != "<redacted>" {
			t.Fatalf("%s = %#v, want <redacted>", key, got[key])
		}
	}
	if got["safe"] != "keep" {
		t.Fatalf("safe = %#v, want ordinary values preserved", got["safe"])
	}
}

// TestRedactExportStringKeepsOrdinaryText guards against a regex that
// swallows prose alongside the secret.
func TestRedactExportStringKeepsOrdinaryText(t *testing.T) {
	input := "please run go test ./... and report the failures"
	if got := redactExportString(input); got != input {
		t.Fatalf("redactExportString() changed ordinary text: %q", got)
	}
}
