package builtin

import (
	"regexp"
	"strings"
)

func redactExportMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if isSensitiveExportKey(k) {
			out[k] = "<redacted>"
			continue
		}
		out[k] = redactExportValue(v)
	}
	return out
}

func redactExportValue(v any) any {
	switch value := v.(type) {
	case string:
		return redactExportString(value)
	case map[string]any:
		return redactExportMap(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = redactExportValue(item)
		}
		return out
	default:
		return value
	}
}

func isSensitiveExportKey(key string) bool {
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

var exportRedactions = []struct {
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

func redactExportString(value string) string {
	out := value
	for _, redaction := range exportRedactions {
		out = redaction.re.ReplaceAllString(out, redaction.repl)
	}
	return out
}
