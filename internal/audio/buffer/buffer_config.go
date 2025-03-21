package buffer

import "time"

// BufferConfig holds all configuration parameters needed by the buffer package
type BufferConfig struct {
	// BirdNET-specific settings
	BirdNETOverlap float64
	SampleRate     int
	BitDepth       int
	BufferSize     int

	// Default values
	DefaultAnalysisDuration time.Duration
	DefaultCaptureDuration  time.Duration
}

// NewDefaultBufferConfig creates a config with sensible defaults
func NewDefaultBufferConfig() *BufferConfig {
	return &BufferConfig{
		BirdNETOverlap:          0.25,
		SampleRate:              48000,
		BitDepth:                16,
		BufferSize:              3 * 48000 * 2, // 3 seconds of 48kHz stereo audio
		DefaultAnalysisDuration: 3 * time.Second,
		DefaultCaptureDuration:  30 * time.Second,
	}
}
