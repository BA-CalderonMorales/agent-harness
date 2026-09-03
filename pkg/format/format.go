// Package format provides small shared formatting helpers.
package format

import "fmt"

// HumanBytes renders a byte count as the largest clean unit
// (0B, 512B, 1.5K, 3.0M, ...).
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	value := float64(n)
	units := []string{"K", "M", "G", "T"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1fP", value)
}
