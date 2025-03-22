package stream

import (
	"context"
	"io"
	"time"
)

// Source represents an audio stream source
type Source interface {
	// Start starts the stream source and returns a reader for audio data
	Start(ctx context.Context) (io.ReadCloser, error)

	// Stop stops the stream source
	Stop() error

	// ID returns a unique identifier for this stream source
	ID() string

	// Name returns a human-readable name for this stream source
	Name() string

	// IsActive returns whether the stream is currently active
	IsActive() bool
}

// DataCallback is a function that processes audio data from a stream
type DataCallback func(sourceID string, data []byte)

// LevelCallback is a function that reports audio level data
type LevelCallback func(level AudioLevelData)

// AudioLevelData describes the audio level of a stream
type AudioLevelData struct {
	Level    int    `json:"level"`    // 0-100
	Clipping bool   `json:"clipping"` // true if clipping is detected
	Source   string `json:"source"`   // Source identifier
	Name     string `json:"name"`     // Human-readable name of the source
}

// AudioFormat defines audio format parameters
type AudioFormat struct {
	SampleRate int
	Channels   int
	BitDepth   int
	Format     string // e.g., "s16le"
}

// Config holds configuration for a stream source
type Config struct {
	ID           string
	Name         string
	URL          string
	Format       AudioFormat
	Transport    string // "tcp", "udp", etc.
	BufferSize   int
	InactiveTime time.Duration // How long to wait before considering a stream inactive
}
