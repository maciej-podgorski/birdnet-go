package file

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ResamplingReaderWrapper wraps an existing Reader to perform automatic resampling
// to the target sample rate (typically 48kHz)
type ResamplingReaderWrapper struct {
	reader          Reader
	resampler       *StreamingResampler
	targetRate      int
	originalRate    int
	needsResampling bool
	debug           bool
}

// NewResamplingReader creates a new resampling reader wrapper
func NewResamplingReader(reader Reader, targetRate int, debug bool) (*ResamplingReaderWrapper, error) {
	if reader == nil {
		return nil, fmt.Errorf("cannot wrap nil reader")
	}

	// Get info from the wrapped reader to determine if resampling is needed
	info, err := reader.GetInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info for resampling: %w", err)
	}

	needsResampling := info.SampleRate != targetRate

	wrapper := &ResamplingReaderWrapper{
		reader:          reader,
		targetRate:      targetRate,
		originalRate:    info.SampleRate,
		needsResampling: needsResampling,
		debug:           debug,
	}

	// Only create a resampler if we actually need it
	if needsResampling {
		if debug {
			fmt.Printf("Creating resampler from %d Hz to %d Hz\n", info.SampleRate, targetRate)
		}
		wrapper.resampler = NewStreamingResampler(info.SampleRate, targetRate)
	}

	return wrapper, nil
}

// Open opens the underlying reader
func (r *ResamplingReaderWrapper) Open(filePath string) error {
	return r.reader.Open(filePath)
}

// Close closes the underlying reader
func (r *ResamplingReaderWrapper) Close() error {
	// Reset the resampler if it exists
	if r.resampler != nil {
		r.resampler.Reset()
	}
	return r.reader.Close()
}

// GetInfo returns metadata about the audio file, adjusted for the target sample rate
func (r *ResamplingReaderWrapper) GetInfo() (Info, error) {
	info, err := r.reader.GetInfo()
	if err != nil {
		return Info{}, err
	}

	// If we're resampling, adjust the sample rate and total samples
	if r.needsResampling {
		// Calculate the new number of total samples based on the ratio
		ratio := float64(r.targetRate) / float64(r.originalRate)
		newTotalSamples := int(float64(info.TotalSamples) * ratio)

		// Create a new info object with adjusted values
		resampledInfo := Info{
			SampleRate:   r.targetRate,
			TotalSamples: newTotalSamples,
			NumChannels:  info.NumChannels,
			BitDepth:     info.BitDepth,
			Duration:     info.Duration, // Duration stays the same
			Format:       info.Format,
			Path:         info.Path,
		}
		return resampledInfo, nil
	}

	return info, nil
}

// ReadChunk reads the next chunk of audio data, resampling if needed
func (r *ResamplingReaderWrapper) ReadChunk(chunkDuration, overlap float64) (resampledChunk Chunk, err error) {
	// Read the original chunk
	chunk, err := r.reader.ReadChunk(chunkDuration, overlap)
	if err != nil {
		return Chunk{}, err
	}

	// If no resampling is needed, return the chunk as is
	if !r.needsResampling {
		return chunk, nil
	}

	// Resample the audio data
	resampledData := r.resampler.Process(chunk.Data)

	// Create new chunk with resampled data
	ratio := float64(r.targetRate) / float64(r.originalRate)
	newSampleOffset := int(float64(chunk.SampleOffset) * ratio)

	resampledChunk = Chunk{
		Data:         resampledData,
		StartTime:    chunk.StartTime,
		SampleOffset: newSampleOffset,
		NumSamples:   len(resampledData),
	}

	if r.debug {
		fmt.Printf("Resampled chunk from %d to %d samples (ratio %.2f)\n",
			len(chunk.Data), len(resampledData), ratio)
	}

	return resampledChunk, nil
}

// ProcessFile processes the entire file in chunks using the provided processor function
func (r *ResamplingReaderWrapper) ProcessFile(ctx context.Context, chunkDuration, overlap float64, processor ChunkProcessor) error {
	// If no resampling is needed, just pass through to the original reader
	if !r.needsResampling {
		return r.reader.ProcessFile(ctx, chunkDuration, overlap, processor)
	}

	// Custom implementation for resampled processing
	for {
		// Check if context has been canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		// Read and resample the next chunk
		chunk, err := r.ReadChunk(chunkDuration, overlap)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading chunk: %w", err)
		}

		// Process the resampled chunk
		if err := processor(chunk); err != nil {
			return fmt.Errorf("processor error: %w", err)
		}
	}

	return nil
}

// Seek positions the reader at the specified offset (in samples)
func (r *ResamplingReaderWrapper) Seek(sampleOffset int) error {
	// If resampling, we need to convert the sample offset to the original rate
	if r.needsResampling {
		ratio := float64(r.originalRate) / float64(r.targetRate)
		originalOffset := int(float64(sampleOffset) * ratio)

		// Reset the resampler since we're changing position
		r.resampler.Reset()

		return r.reader.Seek(originalOffset)
	}

	return r.reader.Seek(sampleOffset)
}

// IsValid validates if the file format is supported and readable
func (r *ResamplingReaderWrapper) IsValid() (bool, error) {
	return r.reader.IsValid()
}
