package commands

import "fmt"

// WorkspaceHandler returns a slash handler for showing workspace info
func WorkspaceHandler(infoFunc func() string) func(string) (string, error) {
	return func(args string) (string, error) {
		return infoFunc(), nil
	}
}

// sprintf is a helper for fmt.Sprintf
func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
