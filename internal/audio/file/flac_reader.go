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

	// New fields for buffered reading
	currentChunk []float32 // Buffer holding all current audio data
	stepSamples  int       // How many samples to advance after each chunk
	chunkSamples int       // Samples in a standard chunk (3 seconds)
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

	// Reset buffer state
	r.currentChunk = nil

	if r.debug {
		fmt.Printf("FLAC Info: %+v\n", r.info)
	}

	return nil
}

// Close closes the file and releases resources.
func (r *FLACReader) Close() error {
	r.isOpen = false
	r.currentChunk = nil
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

// fillBuffer reads more data into the buffer
func (r *FLACReader) fillBuffer(minSamples int) error {
	if r.debug {
		fmt.Printf("DEBUG FLAC Reader: Filling buffer, need %d samples, have %d\n",
			minSamples, len(r.currentChunk))
	}

	// Only read as many as we need
	samplesToRead := minSamples - len(r.currentChunk)
	if samplesToRead <= 0 {
		return nil
	}

	// Get the divisor for normalizing samples
	divisor, err := getAudioDivisor(r.info.BitDepth)
	if err != nil {
		return err
	}

	samplesAdded := 0

	// Read frames until we have enough samples or reach EOF
	for samplesAdded < samplesToRead {
		frame, err := r.decoder.Next()
		if errors.Is(err, io.EOF) {
			if r.debug {
				fmt.Printf("DEBUG FLAC Reader: Reached EOF while filling buffer at position %d, added %d samples\n",
					r.lastPos+samplesAdded, samplesAdded)
			}
			return io.EOF
		} else if err != nil {
			return fmt.Errorf("error reading FLAC frame: %w", err)
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

			r.currentChunk = append(r.currentChunk, float32(sample)/divisor)
			samplesAdded++

			// If we have read enough samples, break
			if samplesAdded >= samplesToRead {
				break
			}
		}
	}

	if r.debug {
		fmt.Printf("DEBUG FLAC Reader: Added %d samples to buffer, total now %d\n",
			samplesAdded, len(r.currentChunk))
	}

	return nil
}

// ReadChunk reads the next chunk of audio data.
func (r *FLACReader) ReadChunk(chunkDuration, overlap float64) (Chunk, error) {
	if !r.isOpen {
		return Chunk{}, errors.New("file not open")
	}

	// Initialize buffer on first call
	if r.currentChunk == nil {
		// Calculate key parameters
		r.chunkSamples = int(chunkDuration * float64(r.info.SampleRate))
		r.stepSamples = int((chunkDuration - overlap) * float64(r.info.SampleRate))

		if r.debug {
			fmt.Printf("DEBUG FLAC Reader: Initializing with chunk size %d samples, step %d samples\n",
				r.chunkSamples, r.stepSamples)
		}

		// Create a large buffer (8 complete chunks worth like in the old code)
		bufferSize := 8 * r.chunkSamples

		// Fill initial buffer
		if err := r.fillBuffer(bufferSize); err != nil && !errors.Is(err, io.EOF) {
			return Chunk{}, err
		}
	}

	// Check if we need more data
	if len(r.currentChunk) < r.chunkSamples {
		// Reached end of file
		if r.lastPos >= r.info.TotalSamples {
			if r.debug {
				fmt.Printf("DEBUG FLAC Reader: Already at end of file, position %d >= total samples %d\n",
					r.lastPos, r.info.TotalSamples)
			}
			return Chunk{}, io.EOF
		}

		// Try to read more data
		if err := r.fillBuffer(r.chunkSamples * 4); err != nil && !errors.Is(err, io.EOF) {
			return Chunk{}, err
		}

		// If we still don't have enough data, check if we have at least 1.5 seconds
		// (matching the behavior in the old code)
		minSamples := int(1.5 * float64(r.info.SampleRate))
		if len(r.currentChunk) < minSamples {
			if r.debug {
				fmt.Printf("DEBUG FLAC Reader: Not enough samples at end of file, have %d, need %d\n",
					len(r.currentChunk), minSamples)
			}
			return Chunk{}, io.EOF
		}
	}

	// Prepare the chunk to return
	var chunk Chunk

	// Calculate start time relative to the beginning of the file
	startTime := time.Duration(float64(r.lastPos) / float64(r.info.SampleRate) * float64(time.Second))

	// Handle the case where we have some data but not a full chunk
	dataLen := len(r.currentChunk)
	if dataLen < r.chunkSamples {
		// Pad with zeros if we don't have a full chunk
		if r.debug {
			fmt.Printf("DEBUG FLAC Reader: Padding chunk with %d zeros (have %d of %d samples)\n",
				r.chunkSamples-dataLen, dataLen, r.chunkSamples)
		}

		paddedData := make([]float32, r.chunkSamples)
		copy(paddedData, r.currentChunk)

		chunk = Chunk{
			Data:         paddedData,
			StartTime:    time.Time{}.Add(startTime),
			SampleOffset: r.lastPos,
			NumSamples:   dataLen,
		}

		// Advance position and clear buffer since we're at EOF
		r.lastPos += r.stepSamples
		r.currentChunk = nil
	} else {
		// Normal case - return full chunk and advance window
		chunk = Chunk{
			Data:         r.currentChunk[:r.chunkSamples],
			StartTime:    time.Time{}.Add(startTime),
			SampleOffset: r.lastPos,
			NumSamples:   r.chunkSamples,
		}

		// Advance our sliding window
		r.lastPos += r.stepSamples

		// Slide the window forward
		if r.stepSamples >= len(r.currentChunk) {
			// Step is larger than our buffer, just clear it
			r.currentChunk = nil
		} else {
			// Normal sliding window
			r.currentChunk = r.currentChunk[r.stepSamples:]
		}
	}

	if r.debug {
		fmt.Printf("DEBUG FLAC Reader: Returned chunk from position %d, advanced to %d\n",
			chunk.SampleOffset, r.lastPos)
	}

	return chunk, nil
}

// ProcessFile processes the entire file in chunks.
func (r *FLACReader) ProcessFile(ctx context.Context, chunkDuration, overlap float64, processor ChunkProcessor) error {
	if !r.isOpen {
		return errors.New("file not open")
	}

	// Reset any existing buffer and position
	r.currentChunk = nil
	r.lastPos = 0

	// Reopen the file to reset the decoder state
	filePath := r.info.Path
	r.Close()

	if err := r.Open(filePath); err != nil {
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
				if r.debug {
					fmt.Println("DEBUG FLAC Reader: Reached EOF, processing complete")
				}
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

	// Reset our buffer and position
	r.currentChunk = nil
	r.lastPos = sampleOffset

	// Reopen the file to reset the decoder state
	filePath := r.info.Path
	r.Close()

	if err := r.Open(filePath); err != nil {
		return err
	}

	// For FLAC, we'll need to read and discard frames until we reach our position
	// This is inefficient but only happens on explicit seeks
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

			// Calculate how many samples are in this frame
			frameSamples := len(frame) / (r.decoder.BitsPerSample / 8) / r.decoder.NChannels

			if frameSamples >= samplesRemaining {
				break
			}
			samplesRemaining -= frameSamples
		}
	}

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
