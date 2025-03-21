package audio

import (
	"runtime"

	"github.com/tphakala/malgo"
)

// GetOSName returns the current operating system name.
func GetOSName() string {
	return runtime.GOOS
}

// GetPlatformDefaultBackend returns the default audio backend for the current platform.
func GetPlatformDefaultBackend() malgo.Backend {
	switch GetOSName() {
	case "linux":
		return malgo.BackendAlsa
	case "windows":
		return malgo.BackendWasapi
	case "darwin":
		return malgo.BackendCoreaudio
	default:
		return malgo.BackendNull
	}
}

// IsUnixOS returns true if the current OS is a Unix-like system.
func IsUnixOS() bool {
	return GetOSName() == "linux" || GetOSName() == "darwin"
}

// IsWindowsOS returns true if the current OS is Windows.
func IsWindowsOS() bool {
	return GetOSName() == "windows"
}

// IsMacOS returns true if the current OS is macOS.
func IsMacOS() bool {
	return GetOSName() == "darwin"
}

// IsLinuxOS returns true if the current OS is Linux.
func IsLinuxOS() bool {
	return GetOSName() == "linux"
}
