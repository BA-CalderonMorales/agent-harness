//go:build !windows

package builtin

import (
	"os/exec"
	"syscall"
)

func configureCommandProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func cancelCommandProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	// The command is the leader of its own group. Killing the group contains
	// descendants started by the shell as well as the shell itself.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}
