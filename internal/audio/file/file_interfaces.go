// Package file provides functionality for reading and processing audio files.
package file

import (
	"context"
	"time"
)

// Format represents the supported audio file formats.
type Format string

const (
	// FormatWAV represents the WAV audio format.
	FormatWAV Format = "wav"

	// FormatFLAC represents the FLAC audio format.
	FormatFLAC Format = "flac"
)

// Info contains metadata about an audio file.
type Info struct {
	SampleRate   int           // Sample rate in Hz
	TotalSamples int           // Total number of samples
	NumChannels  int           // Number of audio channels
	BitDepth     int           // Bit depth (e.g., 16, 24, 32)
	Duration     time.Duration // Duration of the audio file
	Format       Format        // Audio format (WAV, FLAC)
	Path         string        // File path
}

// Chunk represents a segment of audio data.
type Chunk struct {
	Data         []float32 // Audio data normalized to float32 (-1.0 to 1.0)
	StartTime    time.Time // Start time of this chunk relative to the file start
	SampleOffset int       // Sample offset from the beginning of the file
	NumSamples   int       // Number of samples in this chunk
}

// ChunkProcessor is a function that processes audio chunks.
type ChunkProcessor func(chunk Chunk) error

// Reader defines the interface for reading audio files.
type Reader interface {
	// Open opens the audio file and prepares it for reading.
	Open(filePath string) error

	// Close closes the audio file and releases any resources.
	Close() error

	// GetInfo returns metadata about the audio file.
	GetInfo() (Info, error)

	// ReadChunk reads the next chunk of audio data.
	// Returns io.EOF when the end of the file is reached.
	ReadChunk(chunkDuration float64, overlap float64) (Chunk, error)

	// ProcessFile processes the entire file in chunks using the provided processor function.
	ProcessFile(ctx context.Context, chunkDuration float64, overlap float64, processor ChunkProcessor) error

	// Seek positions the reader at the specified offset (in samples).
	Seek(sampleOffset int) error

	// IsValid validates if the file format is supported and readable.
	IsValid() (bool, error)
}

// ReaderFactory creates appropriate readers for different audio formats.
type ReaderFactory interface {
	// CreateReader creates a reader for the specified file.
	CreateReader(filePath string) (Reader, error)
}

// Manager coordinates file operations and provides a simplified API.
type Manager interface {
	// GetFileInfo returns metadata about the specified audio file.
	GetFileInfo(filePath string) (Info, error)

	// ValidateFile checks if the file is a valid, supported audio file.
	ValidateFile(filePath string) error

	// ProcessAudioFile processes an audio file with the provided processor function.
	ProcessAudioFile(ctx context.Context, filePath string, chunkDuration float64, overlap float64, processor ChunkProcessor) error

	// ReadChunks reads all chunks from a file and returns them.
	// Note: Use with caution for large files.
	ReadChunks(filePath string, chunkDuration float64, overlap float64) ([]Chunk, error)
}
