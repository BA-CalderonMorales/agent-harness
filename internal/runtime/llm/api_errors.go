package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIErrorMetadata is the provider's own classification of a failure.
// OpenRouter attaches it to 402/429 responses and mid-stream errors; it
// is what distinguishes a platform free-tier cap from an upstream
// provider at capacity, and a rate limit from a credit rejection.
type APIErrorMetadata struct {
	ErrorType    string `json:"error_type"`
	ProviderCode string `json:"provider_code"`
	LimitSource  string `json:"limit_source"`
	RemedyHint   string `json:"remedy_hint"`
}

// APIError carries the provider's sentence plus the metadata and rate
// limit headers the UI needs to name the actual cause. The old path kept
// only the message ("Rate limit exceeded"), which is the same sentence
// for a free-model daily cap, a busy upstream provider, and a budget
// rejection.
type APIError struct {
	StatusCode int
	Message    string
	Metadata   APIErrorMetadata
	Limit      string
	Remaining  string
	Reset      string
	RetryAfter string
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "provider request failed"
	}
	prefix := "Provider stream error"
	if e.StatusCode > 0 {
		prefix = fmt.Sprintf("LLM API error %d", e.StatusCode)
	}
	result := prefix + ": " + message

	parts := make([]string, 0, 6)
	if e.Metadata.ErrorType != "" {
		parts = append(parts, "type="+e.Metadata.ErrorType)
	}
	if e.Metadata.ProviderCode != "" {
		parts = append(parts, "provider="+e.Metadata.ProviderCode)
	}
	if e.Metadata.LimitSource != "" {
		parts = append(parts, "limit_source="+e.Metadata.LimitSource)
	}
	if e.Limit != "" {
		parts = append(parts, "limit="+e.Limit)
	}
	if e.Remaining != "" {
		parts = append(parts, "remaining="+e.Remaining)
	}
	if e.Reset != "" {
		parts = append(parts, "reset="+e.Reset)
	}
	if e.RetryAfter != "" {
		parts = append(parts, "retry_after="+e.RetryAfter)
	}
	if len(parts) > 0 {
		result += " (" + strings.Join(parts, "; ") + ")"
	}
	if hint := strings.TrimSpace(e.Metadata.RemedyHint); hint != "" {
		result += "\n" + hint
	}
	return result
}

// apiErrorMetadata extracts the metadata object from an error envelope.
// Bodies without that shape simply carry no metadata.
func apiErrorMetadata(body []byte) APIErrorMetadata {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Error) == 0 {
		return APIErrorMetadata{}
	}
	var object struct {
		Metadata APIErrorMetadata `json:"metadata"`
	}
	if err := json.Unmarshal(envelope.Error, &object); err != nil {
		return APIErrorMetadata{}
	}
	return object.Metadata
}

// parseStreamError decodes the error object OpenRouter sends inside an
// SSE chunk when a rate limit lands after streaming has started. It
// accepts both the object and bare-string dialects so an error never
// masquerades as a normal finish.

// apiErrorMessage extracts the human-readable sentence from a provider
// error body. Hosted APIs answer a failed request with a JSON envelope
// ({"error":{"message":"..."}}, {"error":"..."}, {"message":"..."},
// {"detail":"..."}); rendering the envelope verbatim buries the one
// line the user needs under punctuation. Bodies that are not the
// expected shape fall back to the raw text.
func parseStreamError(raw json.RawMessage) *APIError {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var object struct {
		Code     int              `json:"code"`
		Message  string           `json:"message"`
		Metadata APIErrorMetadata `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &object); err == nil && (object.Code != 0 || object.Message != "") {
		return &APIError{StatusCode: object.Code, Message: object.Message, Metadata: object.Metadata}
	}
	var message string
	if err := json.Unmarshal(raw, &message); err == nil && message != "" {
		return &APIError{Message: message}
	}
	return &APIError{Message: string(raw)}
}

func apiErrorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "no response body"
	}

	var envelope struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
		Detail  string          `json:"detail"`
	}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return trimmed
	}

	if len(envelope.Error) > 0 {
		var asString string
		if json.Unmarshal(envelope.Error, &asString) == nil && asString != "" {
			return asString
		}
		var asObject struct {
			Message string `json:"message"`
			Detail  string `json:"detail"`
		}
		if json.Unmarshal(envelope.Error, &asObject) == nil {
			if asObject.Message != "" {
				return asObject.Message
			}
			if asObject.Detail != "" {
				return asObject.Detail
			}
		}
	}
	if envelope.Message != "" {
		return envelope.Message
	}
	if envelope.Detail != "" {
		return envelope.Detail
	}
	return trimmed
}
