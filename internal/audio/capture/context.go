// Package capture provides audio device management and capture functionality.
package capture

import (
	"fmt"
	"sync"

	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/malgo"
)

// MalgoContextAdapter adapts malgo.AllocatedContext to our AudioContext interface.
type MalgoContextAdapter struct {
	context *malgo.AllocatedContext
	mu      sync.Mutex
}

// NewMalgoContextAdapter creates a new adapter for the malgo context.
func NewMalgoContextAdapter(backends []malgo.Backend, config *malgo.ContextConfig, logger func(string)) (audio.AudioContext, error) {
	ctx, err := malgo.InitContext(backends, *config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize malgo context: %w", err)
	}
	return &MalgoContextAdapter{context: ctx}, nil
}

// Devices implements the AudioContext interface.
func (a *MalgoContextAdapter) Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.context.Devices(deviceType)
}

// InitDevice implements the AudioContext interface.
func (a *MalgoContextAdapter) InitDevice(config *malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return malgo.InitDevice(a.context.Context, *config, callbacks)
}

// Uninit implements the AudioContext interface.
func (a *MalgoContextAdapter) Uninit() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.context.Uninit()
}

// GetRawContext returns the underlying malgo.AllocatedContext.
// This is useful for testing and for specific operations that aren't covered by the interface.
func (a *MalgoContextAdapter) GetRawContext() *malgo.AllocatedContext {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.context
}
