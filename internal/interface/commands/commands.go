package commands

// WorkspaceHandler returns a slash handler for showing workspace info
func WorkspaceHandler(infoFunc func() string) func(string) (string, error) {
	return func(args string) (string, error) {
		return infoFunc(), nil
	}
}


