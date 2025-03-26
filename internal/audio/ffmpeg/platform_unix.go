//go:build !windows
// +build !windows

package ffmpeg

import (
	"syscall"
)

// setupProcessGroupUnix sets up process group for Unix systems
func setupProcessGroupUnix(cmd Commander) {
	cmd.SetSysProcAttr(&syscall.SysProcAttr{
		Setpgid: true,
	})
}

// killProcessGroupUnix kills a process and its children on Unix systems
func killProcessGroupUnix(cmd Commander) error {
	if cmd.Process() == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process().Pid, syscall.SIGKILL)
}

// setupProcessGroup is the platform-specific implementation
func setupProcessGroup(cmd Commander) {
	setupProcessGroupUnix(cmd)
}

// killProcessGroup is the platform-specific implementation
func killProcessGroup(cmd Commander) error {
	return killProcessGroupUnix(cmd)
}
