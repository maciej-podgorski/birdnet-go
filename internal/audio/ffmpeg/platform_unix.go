//go:build !windows
// +build !windows

package ffmpeg

import (
	"os/exec"
	"syscall"
)

// setupProcessGroupUnix sets up process group for Unix systems
func setupProcessGroupUnix(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// killProcessGroupUnix kills a process and its children on Unix systems
func killProcessGroupUnix(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// setupProcessGroup is the platform-specific implementation
func setupProcessGroup(cmd *exec.Cmd) {
	setupProcessGroupUnix(cmd)
}

// killProcessGroup is the platform-specific implementation
func killProcessGroup(cmd *exec.Cmd) error {
	return killProcessGroupUnix(cmd)
}
