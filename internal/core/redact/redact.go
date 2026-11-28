// Package redact scrubs credentials, secret references, private keys, and
// local user paths from exported transcripts and structured tool payloads.
//
// Keeping one authoritative redaction table removes drift between /export
// (internal/core/state) and the built-in export tool (internal/runtime/tools/builtin),
// and ensures exact active-secret scrubbing applies uniformly to every export path.
package redact

import (
	"regexp"
	"strings"
	"sync"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
)

var (
	secretMu     sync.RWMutex
	activeSecret string
)

// SetSecret registers the active API key for exact redaction in exports.
// An empty value clears it. Safe for concurrent use.
func SetSecret(secret string) {
	secretMu.Lock()
	activeSecret = secret
	secretMu.Unlock()
}

func currentSecret() string {
	secretMu.RLock()
	s := activeSecret
	secretMu.RUnlock()
	return s
}

var patterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`secret://(?:env|file|cmd):[^\r\n"'\x60]*`), `<secret-ref>`},
	{regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*)\s*[:=]\s*["']?[^"',\s}\]]+["']?`), `$1=<redacted>`},
	{regexp.MustCompile(`(?i)\b(authorization["']?\s*[:=]\s*["']?(?:bearer|basic|token)\s+)[^\s"',;]+`), `$1<redacted>`},
	{regexp.MustCompile(`(?i)\b(x-api-key|x-auth-token)["']?\s*[:=]\s*["']?[^"',\s}\]]+`), `$1=<redacted>`},
	{regexp.MustCompile(`\b(sk-or-v1-[A-Za-z0-9._-]+|sk-ant-[A-Za-z0-9._-]+|sk-proj-[A-Za-z0-9._-]+|sk_live_[A-Za-z0-9]+|sk-[A-Za-z0-9._-]{20,}|github_pat_[A-Za-z0-9_]+|ghp_[A-Za-z0-9_]+|AKIA[0-9A-Z]{16}|xai-[A-Za-z0-9._-]+|AIza[0-9A-Za-z_-]{30,}|hf_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]+|npm_[A-Za-z0-9]{20,}|pypi-[A-Za-z0-9._-]+)\b`), `<redacted>`},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), `<redacted-private-key>`},
	{regexp.MustCompile(`/data/data/com\.termux/files/home(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<termux-home>`},
	{regexp.MustCompile(`/data/data/com\.termux/files/usr(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<termux-prefix>`},
	{regexp.MustCompile(`/(?:home|Users)/[A-Za-z0-9._-]+(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<user-home>`},
	{regexp.MustCompile(`/root(?:/[^\s` + "`" + `"')\]}>,]*)*`), `<user-home>`},
}

// IsSensitiveKey reports whether a map or header key holds credential material.
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "api-key") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "authorization") ||
		strings.Contains(k, "cookie")
}

// String scrubs the active secret, known credential patterns, private keys,
// secret:// references, and local home paths from value.
func String(value string) string {
	out := value
	if secret := currentSecret(); secret != "" {
		out = strings.ReplaceAll(out, secret, "<redacted>")
	}
	for _, p := range patterns {
		out = p.re.ReplaceAllString(out, p.repl)
	}
	return out
}

// Map returns a redacted shallow-to-deep copy of a structured map.
func Map(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if IsSensitiveKey(k) {
			out[k] = "<redacted>"
			continue
		}
		out[k] = Value(v)
	}
	return out
}

// Value recursively redacts strings, maps, and slices.
func Value(v any) any {
	switch value := v.(type) {
	case string:
		return String(value)
	case map[string]any:
		return Map(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = Value(item)
		}
		return out
	default:
		return value
	}
}

// Messages returns a deep-redacted copy of msgs without mutating the input.
func Messages(msgs []types.Message) []types.Message {
	out := make([]types.Message, 0, len(msgs))
	for _, msg := range msgs {
		msg.APIError = String(msg.APIError)
		msg.StopReason = String(msg.StopReason)
		msg.Model = String(msg.Model)
		originalContent := msg.Content
		msg.Content = make([]types.ContentBlock, 0, len(originalContent))
		for _, block := range originalContent {
			switch v := block.(type) {
			case types.TextBlock:
				msg.Content = append(msg.Content, types.TextBlock{Text: String(v.Text)})
			case types.ToolUseBlock:
				msg.Content = append(msg.Content, types.ToolUseBlock{
					ID:    v.ID,
					Name:  v.Name,
					Input: Map(v.Input),
				})
			case types.ToolResultBlock:
				msg.Content = append(msg.Content, types.ToolResultBlock{
					ToolUseID: v.ToolUseID,
					Content:   String(v.Content),
					IsError:   v.IsError,
				})
			case types.ThinkingBlock:
				msg.Content = append(msg.Content, types.ThinkingBlock{
					Thinking:  "<redacted>",
					Signature: "<redacted>",
				})
			default:
				msg.Content = append(msg.Content, block)
			}
		}
		out = append(out, msg)
	}
	return out
}
