package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

const toolResultReceiptSeparator = "\n...\n"

func persistToolResultReceipt(result string, maxReceiptChars int) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for full tool result: %w", err)
	}

	resultDir, err := filepath.Abs(filepath.Join(home, ".agent-harness", "tool-results"))
	if err != nil {
		return "", fmt.Errorf("resolve full tool result directory: %w", err)
	}
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return "", fmt.Errorf("create full tool result directory: %w", err)
	}
	if err := os.Chmod(resultDir, 0o700); err != nil {
		return "", fmt.Errorf("secure full tool result directory: %w", err)
	}

	sum := sha256.Sum256([]byte(result))
	resultPath := filepath.Join(resultDir, fmt.Sprintf("%x.txt", sum))
	receipt, err := boundedToolResultReceipt(result, resultPath, sum, maxReceiptChars)
	if err != nil {
		return "", err
	}
	if err := writeToolResultFile(resultDir, resultPath, []byte(result)); err != nil {
		return "", err
	}
	return receipt, nil
}

func boundedToolResultReceipt(result, resultPath string, sum [sha256.Size]byte, maxChars int) (string, error) {
	metadata := fmt.Sprintf(
		"\n\n[Tool result truncated: original_chars=%d sha256=%x]\n[Full result stored at %s]",
		len(result),
		sum,
		resultPath,
	)
	available := maxChars - len(metadata) - len(toolResultReceiptSeparator)
	if available < 2 {
		return "", fmt.Errorf(
			"tool result budget of %d chars is too small for a durable retrieval receipt",
			maxChars,
		)
	}

	headLimit := (available + 1) / 2
	tailLimit := available - headLimit
	head := validUTF8Prefix(result, headLimit)
	tail := validUTF8Suffix(result, tailLimit)
	return head + toolResultReceiptSeparator + tail + metadata, nil
}

func validUTF8Prefix(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}

func validUTF8Suffix(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	start := len(value) - maxBytes
	for start < len(value) && !utf8.ValidString(value[start:]) {
		start++
	}
	return value[start:]
}

func writeToolResultFile(resultDir, resultPath string, data []byte) error {
	temp, err := os.CreateTemp(resultDir, ".tool-result-*")
	if err != nil {
		return fmt.Errorf("create full tool result file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure full tool result file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write full tool result: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync full tool result: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close full tool result: %w", err)
	}
	if err := os.Rename(tempPath, resultPath); err != nil {
		return fmt.Errorf("publish full tool result: %w", err)
	}
	if err := os.Chmod(resultPath, 0o600); err != nil {
		return fmt.Errorf("secure published full tool result: %w", err)
	}
	return nil
}
