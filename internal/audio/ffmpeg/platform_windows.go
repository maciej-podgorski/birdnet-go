//go:build windows
// +build windows

package ffmpeg

import (
	"fmt"
	"os/exec"
	"syscall"
)

// setupProcessGroupWindows sets up process group for Windows
func setupProcessGroupWindows(cmd Commander) {
	cmd.SetSysProcAttr(&syscall.SysProcAttr{
		CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP in Windows
	})
}

// killProcessGroupWindows kills a process and its children on Windows
func killProcessGroupWindows(cmd Commander) error {
	if cmd.Process() == nil {
		return nil
	}

	// Try using taskkill to terminate the process tree
	taskKill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(cmd.Process().Pid))
	if err := taskKill.Run(); err != nil {
		// Fall back to direct kill if taskkill fails
		return cmd.Process().Kill()
	}
	return nil
}

// setupProcessGroup is the platform-specific implementation
func setupProcessGroup(cmd Commander) {
	setupProcessGroupWindows(cmd)
}

// killProcessGroup is the platform-specific implementation
func killProcessGroup(cmd Commander) error {
	return killProcessGroupWindows(cmd)
}
