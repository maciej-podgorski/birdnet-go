package file

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tphakala/flac"
)

// FLACReader implements the Reader interface for FLAC files.
type FLACReader struct {
	file    *os.File
	decoder *flac.Decoder
	info    Info
	lastPos int // Position of the last read sample
	isOpen  bool
	debug   bool
}

// NewFLACReader creates a new FLAC file reader.
func NewFLACReader(debug bool) *FLACReader {
	return &FLACReader{
		debug: debug,
	}
}

// Open opens the FLAC file and initializes the reader.
func (r *FLACReader) Open(filePath string) error {
	if r.isOpen {
		r.Close()
	}

	var err error
	r.file, err = os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open FLAC file: %w", err)
	}

	r.decoder, err = flac.NewDecoder(r.file)
	if err != nil {
		r.file.Close()
		return fmt.Errorf("failed to create FLAC decoder: %w", err)
	}

	// Calculate duration
	duration := time.Duration(float64(r.decoder.TotalSamples) / float64(r.decoder.SampleRate) * float64(time.Second))

	r.info = Info{
		SampleRate:   r.decoder.SampleRate,
		TotalSamples: int(r.decoder.TotalSamples),
		NumChannels:  r.decoder.NChannels,
		BitDepth:     r.decoder.BitsPerSample,
		Duration:     duration,
		Format:       FormatFLAC,
		Path:         filePath,
	}

	r.lastPos = 0
	r.isOpen = true

	if r.debug {
		fmt.Printf("FLAC Info: %+v\n", r.info)
	}

	return nil
}

// Close closes the file and releases resources.
func (r *FLACReader) Close() error {
	r.isOpen = false
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// GetInfo returns metadata about the audio file.
func (r *FLACReader) GetInfo() (Info, error) {
	if !r.isOpen {
		return Info{}, errors.New("file not open")
	}
	return r.info, nil
}

// ReadChunk reads the next chunk of audio data.
func (r *FLACReader) ReadChunk(chunkDuration float64, overlap float64) (Chunk, error) {
	if !r.isOpen {
		return Chunk{}, errors.New("file not open")
	}

	// Calculate number of samples in this chunk
	chunkSamples := int(chunkDuration * float64(r.info.SampleRate))

	if r.debug {
		fmt.Printf("DEBUG FLAC Reader: Reading chunk of %d samples, current position %d, total samples %d\n",
			chunkSamples, r.lastPos, r.info.TotalSamples)
	}

	// Get the divisor for normalizing samples
	divisor, err := getAudioDivisor(r.info.BitDepth)
	if err != nil {
		return Chunk{}, err
	}

	floatData := make([]float32, 0, chunkSamples)
	samplesRead := 0

	// Read frames until we have enough samples or reach EOF
	for samplesRead < chunkSamples {
		frame, err := r.decoder.Next()
		if errors.Is(err, io.EOF) {
			if r.debug {
				fmt.Printf("DEBUG FLAC Reader: Reached EOF at position %d of %d total samples (read %d samples for this chunk)\n",
					r.lastPos+samplesRead, r.info.TotalSamples, samplesRead)
			}
			break
		} else if err != nil {
			return Chunk{}, fmt.Errorf("error reading FLAC frame: %w", err)
		}

		// Convert bytes to float32 samples
		for i := 0; i < len(frame); i += (r.decoder.BitsPerSample / 8) * r.decoder.NChannels {
			var sample int32

			switch r.decoder.BitsPerSample {
			case 16:
				sample = int32(int16(binary.LittleEndian.Uint16(frame[i:])))
			case 24:
				sample = int32(frame[i]) | int32(frame[i+1])<<8 | int32(frame[i+2])<<16
				// Sign extension for 24-bit
				if sample&0x800000 != 0 {
					sample |= -1 << 24
				}
			case 32:
				sample = int32(binary.LittleEndian.Uint32(frame[i:]))
			}

			floatData = append(floatData, float32(sample)/divisor)
			samplesRead++

			// If we have read enough samples, break
			if samplesRead >= chunkSamples {
				break
			}
		}
	}

	// If we reached EOF and didn't get enough samples, pad with zeros
	if samplesRead < chunkSamples {
		if r.debug {
			fmt.Printf("DEBUG FLAC Reader: Padding chunk with %d zeros (read %d of %d samples)\n",
				chunkSamples-samplesRead, samplesRead, chunkSamples)
		}
		floatData = append(floatData, make([]float32, chunkSamples-samplesRead)...)
	}

	// If we didn't read any samples, return EOF
	if samplesRead == 0 {
		if r.debug {
			fmt.Printf("DEBUG FLAC Reader: No samples read, returning EOF at position %d of %d total samples\n",
				r.lastPos, r.info.TotalSamples)
		}
		return Chunk{}, io.EOF
	}

	// Calculate start time relative to the beginning of the file
	sampleOffset := r.lastPos
	startTime := time.Duration(float64(sampleOffset) / float64(r.info.SampleRate) * float64(time.Second))

	// Update the position for the next read
	// Calculate step size in samples based on chunk duration and overlap in seconds
	stepSamples := int((chunkDuration - overlap) * float64(r.info.SampleRate))
	r.lastPos += stepSamples

	if r.debug {
		fmt.Printf("DEBUG FLAC Reader: Advanced position by %d samples to %d, overlap %.2f seconds\n",
			stepSamples, r.lastPos, overlap)
	}

	// Check if next position would be beyond the file
	if r.lastPos >= r.info.TotalSamples {
		if r.debug {
			fmt.Printf("DEBUG FLAC Reader: Next position %d would exceed total samples %d\n",
				r.lastPos, r.info.TotalSamples)
		}
	}

	return Chunk{
		Data:         floatData,
		StartTime:    time.Time{}.Add(startTime), // Using zero time as base
		SampleOffset: sampleOffset,
		NumSamples:   samplesRead,
	}, nil
}

// ProcessFile processes the entire file in chunks.
func (r *FLACReader) ProcessFile(ctx context.Context, chunkDuration float64, overlap float64, processor ChunkProcessor) error {
	if !r.isOpen {
		return errors.New("file not open")
	}

	// Seek to the beginning of the file
	if err := r.Seek(0); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read next chunk
			chunk, err := r.ReadChunk(chunkDuration, overlap)
			if errors.Is(err, io.EOF) {
				return nil // Reached end of file, processing complete
			}
			if err != nil {
				return err
			}

			// Process the chunk
			if err := processor(chunk); err != nil {
				return err
			}
		}
	}
}

// Seek positions the reader at the specified offset.
func (r *FLACReader) Seek(sampleOffset int) error {
	if !r.isOpen {
		return errors.New("file not open")
	}

	// FLAC decoder doesn't have a direct seek method, so we need to reopen the file
	filePath := r.info.Path
	r.Close()

	if err := r.Open(filePath); err != nil {
		return err
	}

	// For FLAC, we'll need to read and discard frames until we reach our position
	// This is inefficient but works as a basic implementation
	if sampleOffset > 0 {
		samplesRemaining := sampleOffset

		for samplesRemaining > 0 {
			frame, err := r.decoder.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil // Reached EOF during seek
				}
				return fmt.Errorf("error seeking: %w", err)
			}

			// Each frame has frameSampleSize samples per channel
			frameSamples := len(frame) / (r.decoder.BitsPerSample / 8) / r.decoder.NChannels
			if frameSamples >= samplesRemaining {
				break
			}
			samplesRemaining -= frameSamples
		}
	}

	r.lastPos = sampleOffset
	return nil
}

// IsValid validates if the file is a valid FLAC file.
func (r *FLACReader) IsValid() (bool, error) {
	if !r.isOpen {
		return false, errors.New("file not open")
	}

	// Check if the bit depth is supported
	if r.info.BitDepth != 16 && r.info.BitDepth != 24 && r.info.BitDepth != 32 {
		if r.debug {
			fmt.Printf("Unsupported bit depth: %d\n", r.info.BitDepth)
		}
		return false, nil
	}

	// Check if the number of channels is supported
	if r.info.NumChannels != 1 && r.info.NumChannels != 2 {
		if r.debug {
			fmt.Printf("Unsupported number of channels: %d\n", r.info.NumChannels)
		}
		return false, nil
	}

	// Ensure the file has samples
	if r.info.TotalSamples == 0 {
		if r.debug {
			fmt.Println("File contains no samples")
		}
		return false, nil
	}

	return true, nil
}
