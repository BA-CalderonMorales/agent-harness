// Termux-compatible password reading
// Falls back to non-echoed input when terminal manipulation fails

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// Debug output helper
func debugPrintf(format string, args ...interface{}) {
	if os.Getenv("AGENT_HARNESS_VERBOSE") == "1" {
		fmt.Fprintf(os.Stderr, "[debug] "+format, args...)
	}
}

// PromptPassword prompts for a password with masking
// Falls back to plain text input on Termux or when terminal manipulation fails
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Try standard password reading first
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()

	if err == nil {
		// Validate we got something
		pass := strings.TrimSpace(string(password))
		return pass, nil
	}

	// Debug: show error in verbose mode
	if os.Getenv("AGENT_HARNESS_VERBOSE") == "1" {
		fmt.Fprintf(os.Stderr, "[debug] term.ReadPassword failed: %v, using fallback\n", err)
	}

	// Fall back to plain text reading for Termux and other environments
	// where terminal manipulation isn't available
	return promptPasswordFallback()
}

// promptPasswordFallback reads password without terminal manipulation
// Used when term.ReadPassword fails (e.g., in Termux)
func promptPasswordFallback() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// IsTermux returns true if running in Termux environment
func IsTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" ||
		strings.Contains(os.Getenv("HOME"), "com.termux")
}
