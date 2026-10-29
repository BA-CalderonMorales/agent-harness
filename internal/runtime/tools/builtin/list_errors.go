package builtin

import "fmt"

// Directory tools never interpret a path as a shell command. Return an
// execution error so audit records and UI rows agree with the tool result.
func directoryListError(path string, err error) error {
	return fmt.Errorf("cannot list directory %q: %w. Supply a literal directory path, for example {\"path\":\".\"}. To inspect a file, use read with file_path. Shell commands, pipes, redirects, and globs belong in bash's command argument; do not repeat this unchanged path", path, err)
}
