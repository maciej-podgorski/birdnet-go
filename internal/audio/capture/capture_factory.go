// Package capture provides audio device management and capture functionality.
package capture

import (
	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/malgo"
)

// ContextFactory is the factory for creating audio contexts.
type ContextFactory struct {
	debug bool
}

// NewContextFactory creates a new context factory.
func NewContextFactory(debug bool) *ContextFactory {
	return &ContextFactory{
		debug: debug,
	}
}

// CreateAudioContext creates a new audio context.
func (f *ContextFactory) CreateAudioContext(logger func(string)) (audio.AudioContext, error) {
	var backends []malgo.Backend

	// Set an appropriate backend based on the platform
	backend := audio.GetPlatformDefaultBackend()
	if backend != malgo.BackendNull {
		backends = []malgo.Backend{backend}
	}

	return NewMalgoContextAdapter(backends, &malgo.ContextConfig{}, logger)
}

// DefaultAudioContextFactory returns the default factory implementation.
func DefaultAudioContextFactory() audio.AudioContextFactory {
	return &ContextFactory{}
}
