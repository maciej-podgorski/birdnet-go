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
}

// NewReaderFactory creates a new StandardReaderFactory.
func NewReaderFactory(debug bool) ReaderFactory {
	return &StandardReaderFactory{
		Debug: debug,
	}
}

// CreateReader creates an appropriate reader for the given file path.
func (f *StandardReaderFactory) CreateReader(filePath string) (Reader, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".wav":
		return NewWAVReader(f.Debug), nil
	case ".flac":
		return NewFLACReader(f.Debug), nil
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// StandardManager implements the Manager interface.
type StandardManager struct {
	factory ReaderFactory
	debug   bool
}

// NewManager creates a new StandardManager.
func NewManager(debug bool) Manager {
	return &StandardManager{
		factory: NewReaderFactory(debug),
		debug:   debug,
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
func (m *StandardManager) ProcessAudioFile(ctx context.Context, filePath string, chunkDuration float64, overlap float64, processor ChunkProcessor) error {
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
func (m *StandardManager) ReadChunks(filePath string, chunkDuration float64, overlap float64) ([]Chunk, error) {
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

// ConvertToPCM converts the audio file to PCM format.
func (m *StandardManager) ConvertToPCM(filePath string, outputPath string, format Format) error {
	// This would typically use FFmpeg or similar for conversion
	// For now, return not implemented
	return errors.New("conversion not implemented yet")
}
