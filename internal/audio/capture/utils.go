package capture

import (
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/malgo"
)

// HexToASCII converts a hexadecimal string to an ASCII string.
// This is used for converting malgo device IDs to human-readable form.
func HexToASCII(hexStr string) (string, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// TestDevice tests if a device can be initialized and started.
// Returns true if the device is working, false otherwise.
func TestDevice(ctx audio.AudioContext, deviceID string, sampleRate, channels uint32) bool {
	// Get the device info
	malgoInfos, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return false
	}

	// Find the device
	var devicePointer interface{}
	var found bool

	for i := range malgoInfos {
		decodedID, err := HexToASCII(malgoInfos[i].ID.String())
		if err != nil {
			continue
		}

		if decodedID == deviceID ||
			malgoInfos[i].Name() == deviceID ||
			strings.Contains(malgoInfos[i].Name(), deviceID) ||
			strings.Contains(decodedID, deviceID) {
			devicePointer = malgoInfos[i].ID.Pointer()
			found = true
			break
		}
	}

	if !found {
		return false
	}

	// Create device config
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = channels
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Alsa.NoMMap = 1

	// Set the device ID based on the pointer type
	switch ptr := devicePointer.(type) {
	case unsafe.Pointer:
		deviceConfig.Capture.DeviceID = ptr
	case uintptr:
		// This unsafe.Pointer conversion is required for miniaudio C API compatibility.
		// The deviceInfo.Pointer comes from the malgo library and represents a hardware
		// resource that is managed by the underlying C library.
		deviceConfig.Capture.DeviceID = unsafe.Pointer(ptr)
	default:
		return false
	}

	// Try to initialize the device
	device, err := ctx.InitDevice(&deviceConfig, malgo.DeviceCallbacks{})
	if err != nil {
		return false
	}
	defer device.Uninit()

	// Try to start the device
	if err := device.Start(); err != nil {
		return false
	}

	// Stop the device
	_ = device.Stop()
	return true
}

// ValidateAudioDevice checks if the configured audio source is available and working.
func ValidateAudioDevice(deviceID string, ctx audio.AudioContext, sampleRate, channels uint32) error {
	if deviceID == "" {
		return nil
	}

	// Get list of capture devices
	infos, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return fmt.Errorf("failed to get capture devices: %w", err)
	}

	// Filter to get only hardware devices to check if any are available
	hardwareDeviceCount := 0
	for i := range infos {
		decodedID, err := HexToASCII(infos[i].ID.String())
		if err != nil {
			continue
		}

		if isHardwareDevice(decodedID) {
			hardwareDeviceCount++
		}

		// Check if this is the device we're looking for
		if matchesDeviceSettings(decodedID, infos[i].Name(), deviceID) {
			// Test the device
			if TestDevice(ctx, deviceID, sampleRate, channels) {
				return nil
			}
			return fmt.Errorf("configured audio device '%s' failed hardware test", deviceID)
		}
	}

	if hardwareDeviceCount == 0 {
		return fmt.Errorf("no hardware audio capture devices found")
	}

	return fmt.Errorf("configured audio device '%s' not found", deviceID)
}

// isHardwareDevice checks if a device ID indicates a hardware device.
func isHardwareDevice(deviceID string) bool {
	if runtime.GOOS == "linux" {
		return strings.Contains(deviceID, ":") && strings.Contains(deviceID, ",")
	}
	return true
}

// matchesDeviceSettings checks if a device matches the configured ID.
func matchesDeviceSettings(decodedID, deviceName, configuredID string) bool {
	if runtime.GOOS == "windows" && configuredID == "sysdefault" {
		// On Windows, there is no "sysdefault" device, use default device
		return true
	}

	// Check if decoded ID or device name matches the configured ID
	return decodedID == configuredID ||
		deviceName == configuredID ||
		strings.Contains(deviceName, configuredID) ||
		strings.Contains(decodedID, configuredID)
}

// ConvertMalgoDeviceInfo converts malgo.DeviceInfo to audio.DeviceInfo.
func ConvertMalgoDeviceInfo(info *malgo.DeviceInfo) (audio.DeviceInfo, error) {
	decodedID, err := HexToASCII(info.ID.String())
	if err != nil {
		return audio.DeviceInfo{}, err
	}

	return audio.DeviceInfo{
		ID:       decodedID,
		Name:     info.Name(),
		IsInput:  true,
		Channels: conf.NumChannels, // TODO: Check if this is correct
	}, nil
}
