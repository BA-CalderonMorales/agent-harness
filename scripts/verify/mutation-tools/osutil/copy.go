// Package osutil supplies the portable file-copy operation required by the
// pinned mutation runner. No Unix process or resource-limit APIs are needed.
package osutil

import (
	"io"
	"os"
)

func CopyFile(source, destination string) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := input.Close(); err == nil {
			err = closeErr
		}
	}()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Chmod(info.Mode().Perm())
}
