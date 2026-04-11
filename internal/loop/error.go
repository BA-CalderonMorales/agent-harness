package loop

import (
	"errors"
	"fmt"
)

// LoopError is a structured error for loop operations.
// It carries categorization for retry decisions and user messages.
type LoopError struct {
	Code       string // Machine-readable error code
	Message    string // Human-readable message
	Cause      error  // Underlying error
	Category   ErrorCategory
	Retryable  bool
	ToolName   string // Which tool caused the error, if any
}

// ErrorCategory classifies errors for handling decisions.
type ErrorCategory string

const (
	ErrCategoryValidation   ErrorCategory = "validation"
	ErrCategoryPermission   ErrorCategory = "permission"
	ErrCategoryExecution    ErrorCategory = "execution"
	ErrCategoryNetwork      ErrorCategory = "network"
	ErrCategoryLLM          ErrorCategory = "llm"
	ErrCategoryContext      ErrorCategory = "context"
	ErrCategoryToolNotFound ErrorCategory = "tool_not_found"
	ErrCategoryUnknown      ErrorCategory = "unknown"
)

// Error implements the error interface.
func (e LoopError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As.
func (e LoopError) Unwrap() error {
	return e.Cause
}

// NewLoopError creates a basic loop error.
func NewLoopError(code, message string) LoopError {
	return LoopError{
		Code:     code,
		Message:  message,
		Category: ErrCategoryUnknown,
	}
}

// WrapError wraps an existing error with loop context.
func WrapError(code string, err error) LoopError {
	return LoopError{
		Code:     code,
		Message:  err.Error(),
		Cause:    err,
		Category: ClassifyError(err),
	}
}

// ClassifyError determines the category of an error.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrCategoryUnknown
	}

	errStr := err.Error()

	switch {
	case containsAny(errStr, "permission", "denied", "unauthorized"):
		return ErrCategoryPermission
	case containsAny(errStr, "validation", "invalid", "required"):
		return ErrCategoryValidation
	case containsAny(errStr, "network", "timeout", "connection", "dial"):
		return ErrCategoryNetwork
	case containsAny(errStr, "max_tokens", "context length", "prompt_too_long"):
		return ErrCategoryContext
	case containsAny(errStr, "not found", "no such tool"):
		return ErrCategoryToolNotFound
	case containsAny(errStr, "llm", "api error", "rate limit"):
		return ErrCategoryLLM
	default:
		return ErrCategoryExecution
	}
}

// IsRetryable determines if an error warrants a retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's already a LoopError
	var loopErr LoopError
	if errors.As(err, &loopErr) {
		return loopErr.Retryable
	}

	// Classify and decide
	cat := ClassifyError(err)
	switch cat {
	case ErrCategoryNetwork, ErrCategoryLLM:
		return true
	case ErrCategoryContext:
		// Context errors might be recoverable with compaction
		return true
	default:
		return false
	}
}

// RecoverableError indicates an error that can be recovered from.
type RecoverableError struct {
	Inner  error
	Reason string // "max_output_tokens", "prompt_too_long", etc.
}

func (e *RecoverableError) Error() string {
	return fmt.Sprintf("recoverable (%s): %v", e.Reason, e.Inner)
}

func (e *RecoverableError) Unwrap() error {
	return e.Inner
}

// IsRecoverable checks if an error is a recoverable error.
func IsRecoverable(err error) (*RecoverableError, bool) {
	var rec *RecoverableError
	if errors.As(err, &rec) {
		return rec, true
	}

	// Check error strings for known recoverable patterns
	errStr := err.Error()
	if containsAny(errStr, "max_output_tokens", "max_tokens") {
		return &RecoverableError{Inner: err, Reason: "max_output_tokens"}, true
	}
	if containsAny(errStr, "prompt_too_long", "context length") {
		return &RecoverableError{Inner: err, Reason: "prompt_too_long"}, true
	}

	return nil, false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
