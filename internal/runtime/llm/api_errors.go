package llm

import (
	"encoding/json"
	"strings"
)

// apiErrorMessage extracts the human-readable sentence from a provider
// error body. Hosted APIs answer a failed request with a JSON envelope
// ({"error":{"message":"..."}}, {"error":"..."}, {"message":"..."},
// {"detail":"..."}); rendering the envelope verbatim buries the one
// line the user needs under punctuation. Bodies that are not the
// expected shape fall back to the raw text.
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
