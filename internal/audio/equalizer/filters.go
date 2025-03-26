// Package audio provides core functionality for audio processing in BirdNET-Go.
package equalizer

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/tphakala/birdnet-go/internal/conf"
)

// Global variables for filter chain and mutex
var (
	filterChain *FilterChain
	filterMutex sync.RWMutex
)

// InitializeFilterChain sets up the initial filter chain based on settings
func InitializeFilterChain(settings *conf.Settings) error {
	filterMutex.Lock()
	defer filterMutex.Unlock()

	// Create a new filter chain
	filterChain = NewFilterChain()

	// If equalizer is enabled in settings, add filters
	if settings.Realtime.Audio.Equalizer.Enabled {
		for _, filterConfig := range settings.Realtime.Audio.Equalizer.Filters {
			// Create and add each filter
			filter, err := createFilter(filterConfig, float64(conf.SampleRate))
			if err != nil {
				return err
			}
			// Note: filter can be nil if passes <= 0 (disabled)
			if filter != nil {
				if err := filterChain.AddFilter(filter); err != nil {
					return fmt.Errorf("failed to add audio EQ filter: %w", err)
				}
			}
		}
	}

	return nil
}

// UpdateFilterChain updates the filter chain based on new settings
func UpdateFilterChain(settings *conf.Settings) error {
	filterMutex.Lock()
	defer filterMutex.Unlock()

	// Create a new filter chain
	newFilterChain := NewFilterChain()

	// If equalizer is enabled, add filters
	if settings.Realtime.Audio.Equalizer.Enabled {
		for _, filterConfig := range settings.Realtime.Audio.Equalizer.Filters {
			// Create and add each filter
			filter, err := createFilter(filterConfig, float64(conf.SampleRate))
			if err != nil {
				return err
			}
			// Note: filter can be nil if passes <= 0 (disabled)
			if filter != nil {
				if err := newFilterChain.AddFilter(filter); err != nil {
					return fmt.Errorf("failed to add audio EQ filter: %w", err)
				}
			}
		}
	}

	// Replace the old filter chain with the new one
	filterChain = newFilterChain
	return nil
}

// ApplyFilters applies the audio filters to the provided audio samples
func ApplyFilters(samples []byte) error {
	filterMutex.RLock()
	defer filterMutex.RUnlock()

	if filterChain == nil {
		return nil // No filters to apply
	}

	// If no filters, return early
	if filterChain.Length() == 0 {
		return nil // No filters to apply
	}

	// Convert byte slice to float64 slice
	floatSamples := make([]float64, len(samples)/2)
	for i := 0; i < len(samples); i += 2 {
		floatSamples[i/2] = float64(int16(binary.LittleEndian.Uint16(samples[i:]))) / 32768.0
	}

	// Apply filters to the float samples in batch
	filterChain.ApplyBatch(floatSamples)

	// Convert back to byte slice
	for i, sample := range floatSamples {
		intSample := int16(sample * 32767.0)
		binary.LittleEndian.PutUint16(samples[i*2:], uint16(intSample))
	}

	return nil
}

// createFilter creates an audio filter based on the provided configuration
func createFilter(config conf.EqualizerFilter, sampleRate float64) (*Filter, error) {
	// If passes is 0 or less, return nil without an error (filter is off)
	if config.Passes <= 0 {
		return nil, nil
	}

	// Create different types of filters based on the configuration
	switch config.Type {
	case "LowPass":
		return NewLowPass(sampleRate, config.Frequency, config.Q, config.Passes)
	case "HighPass":
		return NewHighPass(sampleRate, config.Frequency, config.Q, config.Passes)
	case "AllPass":
		return NewAllPass(sampleRate, config.Frequency, config.Q, config.Passes)
	case "BandPass":
		return NewBandPass(sampleRate, config.Frequency, config.Width, config.Passes)
	case "BandReject":
		return NewBandReject(sampleRate, config.Frequency, config.Width, config.Passes)
	case "LowShelf":
		return NewLowShelf(sampleRate, config.Frequency, config.Q, config.Gain, config.Passes)
	case "HighShelf":
		return NewHighShelf(sampleRate, config.Frequency, config.Q, config.Gain, config.Passes)
	case "Peaking":
		return NewPeaking(sampleRate, config.Frequency, config.Width, config.Gain, config.Passes)
	default:
		return nil, fmt.Errorf("unsupported filter type: %s", config.Type)
	}
}
