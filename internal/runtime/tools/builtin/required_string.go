package builtin

import "fmt"

// Missing/null/wrong-type strings must never silently become intentional
// empty content. An explicitly supplied empty string remains valid.
func requiredStringArgument(input map[string]any, field, tool string) (string, error) {
	value, ok := input[field].(string)
	if !ok {
		return "", fmt.Errorf("%s requires %q as a string; the argument is missing or has the wrong type", tool, field)
	}
	return value, nil
}
