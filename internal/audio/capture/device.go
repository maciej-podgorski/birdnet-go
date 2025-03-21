package capture

import (
	"sync"

	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/malgo"
)

// MalgoDeviceAdapter adapts malgo.Device to our AudioDevice interface.
type MalgoDeviceAdapter struct {
	device *malgo.Device
	mu     sync.Mutex
}

// NewMalgoDeviceAdapter creates a new adapter for the malgo device.
func NewMalgoDeviceAdapter(device *malgo.Device) audio.AudioDevice {
	return &MalgoDeviceAdapter{device: device}
}

// Start implements the AudioDevice interface.
func (a *MalgoDeviceAdapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.device.Start()
}

// Stop implements the AudioDevice interface.
func (a *MalgoDeviceAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.device.Stop()
}

// Uninit implements the AudioDevice interface.
func (a *MalgoDeviceAdapter) Uninit() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.device.Uninit()
	return nil
}

// GetRawDevice returns the underlying malgo.Device.
// This is useful for testing and for specific operations that aren't covered by the interface.
func (a *MalgoDeviceAdapter) GetRawDevice() *malgo.Device {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.device
}
