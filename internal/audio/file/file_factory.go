package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StandardReaderFactory implements the ReaderFactory interface.
type StandardReaderFactory struct {
	// Configuration options could be added here
	Debug bool
	// Default target sample rate for resampling (typically 48kHz)
	TargetSampleRate int
	// Whether to enable automatic resampling
	EnableResampling bool
}

// NewReaderFactory creates a new StandardReaderFactory.
func NewReaderFactory(debug bool) ReaderFactory {
	return &StandardReaderFactory{
		Debug:            debug,
		TargetSampleRate: 48000, // Default to 48kHz
		EnableResampling: true,  // Enable resampling by default
	}
}

// NewReaderFactoryWithOptions creates a new StandardReaderFactory with custom options.
func NewReaderFactoryWithOptions(debug bool, targetSampleRate int, enableResampling bool) ReaderFactory {
	return &StandardReaderFactory{
		Debug:            debug,
		TargetSampleRate: targetSampleRate,
		EnableResampling: enableResampling,
	}
}

// CreateReader creates an appropriate reader for the given file path.
func (f *StandardReaderFactory) CreateReader(filePath string) (Reader, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	var baseReader Reader
	switch ext {
	case ".wav":
		baseReader = NewWAVReader(f.Debug)
	case ".flac":
		baseReader = NewFLACReader(f.Debug)
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}

	// Open the reader to check if we need resampling
	if err := baseReader.Open(filePath); err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// If resampling is enabled, wrap the reader with a resampling wrapper
	if f.EnableResampling {
		info, err := baseReader.GetInfo()
		if err != nil {
			baseReader.Close()
			return nil, fmt.Errorf("failed to get audio info: %w", err)
		}

		// If the sample rate doesn't match the target, create a resampling wrapper
		if info.SampleRate != f.TargetSampleRate {
			if f.Debug {
				fmt.Printf("Creating resampling wrapper for %s (from %d Hz to %d Hz)\n",
					filepath.Base(filePath), info.SampleRate, f.TargetSampleRate)
			}

			// Close the base reader - it will be reopened by the wrapper as needed
			baseReader.Close()

			wrapper, err := NewResamplingReader(baseReader, f.TargetSampleRate, f.Debug)
			if err != nil {
				return nil, fmt.Errorf("failed to create resampling wrapper: %w", err)
			}
			return wrapper, nil
		}

		// If we don't need resampling, close and reopen to reset position
		baseReader.Close()
		if err := baseReader.Open(filePath); err != nil {
			return nil, fmt.Errorf("failed to reopen file: %w", err)
		}
	}

	return baseReader, nil
}

// StandardManager implements the Manager interface.
type StandardManager struct {
	factory ReaderFactory
	debug   bool
	// Default target sample rate for resampling
	targetSampleRate int
	// Whether to enable automatic resampling
	enableResampling bool
}

// NewManager creates a new StandardManager with default options (resampling enabled, 48kHz).
func NewManager(debug bool) Manager {
	return &StandardManager{
		factory:          NewReaderFactory(debug),
		debug:            debug,
		targetSampleRate: 48000,
		enableResampling: true,
	}
}

// NewManagerWithOptions creates a new StandardManager with custom resampling options.
func NewManagerWithOptions(debug bool, targetSampleRate int, enableResampling bool) Manager {
	return &StandardManager{
		factory:          NewReaderFactoryWithOptions(debug, targetSampleRate, enableResampling),
		debug:            debug,
		targetSampleRate: targetSampleRate,
		enableResampling: enableResampling,
	}
}

// GetFileInfo returns metadata about the specified audio file.
func (m *StandardManager) GetFileInfo(filePath string) (Info, error) {
	reader, err := m.factory.CreateReader(filePath)
	if err != nil {
		return Info{}, err
	}
	defer reader.Close()

	if err := reader.Open(filePath); err != nil {
		return Info{}, fmt.Errorf("failed to open file: %w", err)
	}

	return reader.GetInfo()
}

// ValidateFile checks if the file is a valid, supported audio file.
func (m *StandardManager) ValidateFile(filePath string) error {
	// Check if file exists and is not a directory
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error accessing file %s: %w", filepath.Base(filePath), err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("the path %s is a directory, not a file", filepath.Base(filePath))
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("file %s is empty (0 bytes)", filepath.Base(filePath))
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".wav" && ext != ".flac" {
		return fmt.Errorf("unsupported audio format: %s", ext)
	}

	// Try to open and validate the file
	reader, err := m.factory.CreateReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := reader.Open(filePath); err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	valid, err := reader.IsValid()
	if err != nil {
		return fmt.Errorf("error validating file: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid audio file %s", filepath.Base(filePath))
	}

	return nil
}

// ProcessAudioFile processes an audio file with the provided processor function.
func (m *StandardManager) ProcessAudioFile(ctx context.Context, filePath string, chunkDuration, overlap float64, processor ChunkProcessor) error {
	reader, err := m.factory.CreateReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := reader.Open(filePath); err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	if m.debug {
		info, _ := reader.GetInfo()
		fmt.Printf("DEBUG Manager: Starting processing of file %s with chunk duration %.2f, overlap %.2f\n",
			filepath.Base(filePath), chunkDuration, overlap)
		fmt.Printf("DEBUG Manager: File info: %d Hz, %d channels, %d bit depth, %d total samples\n",
			info.SampleRate, info.NumChannels, info.BitDepth, info.TotalSamples)
	}

	// Count how many chunks are processed
	chunkCount := 0

	// Create a wrapper for the processor to count chunks
	wrappedProcessor := func(chunk Chunk) error {
		chunkCount++
		if m.debug && (chunkCount == 1 || chunkCount%10 == 0) {
			fmt.Printf("DEBUG Manager: Processing chunk #%d (sample offset: %d)\n",
				chunkCount, chunk.SampleOffset)
		}
		return processor(chunk)
	}

	err = reader.ProcessFile(ctx, chunkDuration, overlap, wrappedProcessor)

	if m.debug {
		if err != nil {
			fmt.Printf("DEBUG Manager: File processing completed with error: %v after %d chunks\n", err, chunkCount)
		} else {
			fmt.Printf("DEBUG Manager: File processing completed successfully, processed %d chunks\n", chunkCount)
		}
	}

	return err
}

// ReadChunks reads all chunks from a file and returns them.
func (m *StandardManager) ReadChunks(filePath string, chunkDuration, overlap float64) ([]Chunk, error) {
	reader, err := m.factory.CreateReader(filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	if err := reader.Open(filePath); err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	var chunks []Chunk

	for {
		chunk, err := reader.ReadChunk(chunkDuration, overlap)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return chunks, err
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
