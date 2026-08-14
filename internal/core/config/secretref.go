// Secret reference resolution for API keys.
//
// The harness keeps credentials out of plaintext config by default, but
// teams often want to source keys from their own secrets manager. A
// config value that starts with "secret://" is resolved at boot through
// a pluggable backend; anything else passes through unchanged:
//
//	secret://env:NAME          value of the NAME environment variable
//	secret://file:PATH         first non-empty line of the file at PATH
//	secret://cmd:COMMAND       first non-empty line of COMMAND's stdout
//
// The command backend wraps any external manager (gcloud secrets,
// ansible-vault, coder secrets, HashiCorp Vault, ...) without native
// SDKs, so the harness stays provider-agnostic:
//
//	api_key: "secret://cmd:gcloud secrets versions access latest --secret=openai-key"
//
// Unknown schemes return an error instead of leaking the literal
// reference into a request header.

package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	secretRefPrefix = "secret://"
	// cmdTimeout caps a secret backend command; a hung manager must not
	// hang boot.
	cmdTimeout = 30 * time.Second
	// maxSecretBytes caps backend output; a runaway dump must not flood
	// memory or leak into diagnostics.
	maxSecretBytes = 64 << 10
)

// IsSecretRef reports whether the value is a credential reference.
func IsSecretRef(value string) bool {
	return strings.HasPrefix(value, secretRefPrefix)
}

// ResolveSecret resolves a credential reference to a concrete secret.
// Plain values are returned unchanged.
//
// Error messages are sanitized: the reference itself (which can embed
// secret-manager arguments, paths, or tokens) is never echoed back.
func ResolveSecret(value string) (string, error) {
	if !IsSecretRef(value) {
		return value, nil
	}

	ref := strings.TrimPrefix(value, secretRefPrefix)
	scheme, rest, found := strings.Cut(ref, ":")
	if !found || rest == "" {
		return "", fmt.Errorf("malformed secret reference (want secret://scheme:value)")
	}

	var resolved string
	switch scheme {
	case "env":
		resolved = os.Getenv(rest)
	case "file":
		data, err := os.ReadFile(rest)
		if err != nil {
			return "", fmt.Errorf("secret file backend: %w", err)
		}
		resolved = firstLine(string(data))
	case "cmd":
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sh", "-c", rest).Output()
		if err != nil {
			return "", fmt.Errorf("secret command backend: %w", err)
		}
		resolved = firstLine(string(out))
	default:
		return "", fmt.Errorf("unknown secret backend %q (supported: env, file, cmd)", scheme)
	}

	// A backend that resolves to nothing is a misconfiguration, not a
	// valid key: fail loudly instead of sending an empty Authorization.
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("secret %s backend resolved to an empty value", scheme)
	}
	return resolved, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if len(s) > maxSecretBytes {
		return s[:maxSecretBytes]
	}
	return s
}
