package tui

import "fmt"

func unreadableSessionWarning(count int) string {
	if count == 1 {
		return "1 saved session could not be read; other sessions remain available."
	}
	return fmt.Sprintf("%d saved sessions could not be read; other sessions remain available.", count)
}
