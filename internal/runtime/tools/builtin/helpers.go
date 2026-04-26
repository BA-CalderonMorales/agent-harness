package builtin

import (
	"encoding/json"
)

// getString extracts a string value from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getNumber extracts a numeric value from a map, accepting int, float64,
// and json.Number. This prevents silent fallback to defaults when callers
// pass integers instead of float64 (e.g. from typed structs or JSON
// unmarshaling with UseNumber).
func getNumber(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}

	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return f, true
		}
		// Try parsing as int64 fallback
		if i, err := n.Int64(); err == nil {
			return float64(i), true
		}
	}

	return 0, false
}

// getInt extracts an int value from a map using getNumber.
func getInt(m map[string]any, key string) int {
	if n, ok := getNumber(m, key); ok {
		return int(n)
	}
	return 0
}
