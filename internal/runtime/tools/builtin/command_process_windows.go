//go:build windows

package builtin

import "os/exec"

func configureCommandProcess(cmd *exec.Cmd) {}

func cancelCommandProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
