package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// WAVReader implements the Reader interface for WAV files.
type WAVReader struct {
	file    *os.File
	decoder *wav.Decoder
	info    Info
	lastPos int // Position of the last read sample
	isOpen  bool
	debug   bool

	// New fields for buffered reading
	currentChunk []float32 // Buffer holding all current audio data
	stepSamples  int       // How many samples to advance after each chunk
	chunkSamples int       // Samples in a standard chunk (3 seconds)
}

// NewWAVReader creates a new WAV file reader.
func NewWAVReader(debug bool) *WAVReader {
	return &WAVReader{
		debug: debug,
	}
}

// Open opens the WAV file and initializes the reader.
func (r *WAVReader) Open(filePath string) error {
	if r.isOpen {
		r.Close()
	}

	var err error
	r.file, err = os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open WAV file: %w", err)
	}

	r.decoder = wav.NewDecoder(r.file)
	r.decoder.ReadInfo()

	if !r.decoder.IsValidFile() {
		r.file.Close()
		return errors.New("invalid WAV file format")
	}

	// Get file size in bytes
	fileInfo, err := r.file.Stat()
	if err != nil {
		r.file.Close()
		return err
	}

	// Calculate total samples
	bytesPerSample := int(r.decoder.BitDepth / 8)
	totalSamples := int(fileInfo.Size()) / bytesPerSample / int(r.decoder.NumChans)

	// Calculate duration
	duration := time.Duration(float64(totalSamples) / float64(r.decoder.SampleRate) * float64(time.Second))

	r.info = Info{
		SampleRate:   int(r.decoder.SampleRate),
		TotalSamples: totalSamples,
		NumChannels:  int(r.decoder.NumChans),
		BitDepth:     int(r.decoder.BitDepth),
		Duration:     duration,
		Format:       FormatWAV,
		Path:         filePath,
	}

	r.lastPos = 0
	r.isOpen = true

	// Reset buffer state
	r.currentChunk = nil

	if r.debug {
		fmt.Printf("WAV Info: %+v\n", r.info)
	}

	return nil
}

// Close closes the file and releases resources.
func (r *WAVReader) Close() error {
	r.isOpen = false
	r.currentChunk = nil
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// GetInfo returns metadata about the audio file.
func (r *WAVReader) GetInfo() (Info, error) {
	if !r.isOpen {
		return Info{}, errors.New("file not open")
	}
	return r.info, nil
}

// getAudioDivisor returns the appropriate divisor for normalizing samples based on bit depth.
func getAudioDivisor(bitDepth int) (float32, error) {
	switch bitDepth {
	case 16:
		return 32768.0, nil
	case 24:
		return 8388608.0, nil
	case 32:
		return 2147483648.0, nil
	default:
		return 0, fmt.Errorf("unsupported bit depth: %d", bitDepth)
	}
}

// fillBuffer reads more data into the buffer
func (r *WAVReader) fillBuffer(minSamples int) error {
	if r.debug {
		fmt.Printf("DEBUG WAV Reader: Filling buffer, need %d samples, have %d\n",
			minSamples, len(r.currentChunk))
	}

	// Only read as many as we need
	samplesToRead := minSamples - len(r.currentChunk)
	if samplesToRead <= 0 {
		return nil
	}

	// Create a buffer for reading
	buf := &audio.IntBuffer{
		Data:   make([]int, samplesToRead),
		Format: &audio.Format{SampleRate: r.info.SampleRate, NumChannels: r.info.NumChannels},
	}

	// Read samples from the decoder
	n, err := r.decoder.PCMBuffer(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("error reading WAV data: %w", err)
	}

	// If we read data, convert and add to our buffer
	if n > 0 {
		// Get the divisor for normalizing samples
		divisor, err := getAudioDivisor(r.info.BitDepth)
		if err != nil {
			return err
		}

		// Convert int samples to float32 and add to current chunk
		floatData := make([]float32, n)
		for i := 0; i < n; i++ {
			floatData[i] = float32(buf.Data[i]) / divisor
		}

		r.currentChunk = append(r.currentChunk, floatData...)

		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Added %d samples to buffer, total now %d\n",
				n, len(r.currentChunk))
		}
	}

	return err // Return EOF if we hit the end
}

// ReadChunk reads the next chunk of audio data.
func (r *WAVReader) ReadChunk(chunkDuration float64, overlap float64) (Chunk, error) {
	if !r.isOpen {
		return Chunk{}, errors.New("file not open")
	}

	// Initialize buffer on first call
	if r.currentChunk == nil {
		// Calculate key parameters
		r.chunkSamples = int(chunkDuration * float64(r.info.SampleRate))
		r.stepSamples = int((chunkDuration - overlap) * float64(r.info.SampleRate))

		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Initializing with chunk size %d samples, step %d samples\n",
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
				fmt.Printf("DEBUG WAV Reader: Already at end of file, position %d >= total samples %d\n",
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
				fmt.Printf("DEBUG WAV Reader: Not enough samples at end of file, have %d, need %d\n",
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
			fmt.Printf("DEBUG WAV Reader: Padding chunk with %d zeros (have %d of %d samples)\n",
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
		fmt.Printf("DEBUG WAV Reader: Returned chunk from position %d, advanced to %d\n",
			chunk.SampleOffset, r.lastPos)
	}

	return chunk, nil
}

// ProcessFile processes the entire file in chunks.
func (r *WAVReader) ProcessFile(ctx context.Context, chunkDuration float64, overlap float64, processor ChunkProcessor) error {
	if !r.isOpen {
		return errors.New("file not open")
	}

	// Reset any existing buffer and position
	r.currentChunk = nil
	r.lastPos = 0

	// Seek to the beginning of the file and reset decoder
	r.file.Seek(0, io.SeekStart)
	r.decoder = wav.NewDecoder(r.file)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read next chunk
			chunk, err := r.ReadChunk(chunkDuration, overlap)
			if errors.Is(err, io.EOF) {
				if r.debug {
					fmt.Println("DEBUG WAV Reader: Reached EOF, processing complete")
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
func (r *WAVReader) Seek(sampleOffset int) error {
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

	// Now we need to skip forward to our position
	if sampleOffset > 0 {
		// This is inefficient, but only happens on explicit seeks
		chunkSize := 8192 // Read in larger chunks for skipping
		buf := &audio.IntBuffer{
			Data:   make([]int, chunkSize),
			Format: &audio.Format{SampleRate: r.info.SampleRate, NumChannels: r.info.NumChannels},
		}

		samplesRemaining := sampleOffset
		for samplesRemaining > 0 {
			toRead := min(chunkSize, samplesRemaining)
			buf.Data = buf.Data[:toRead]

			n, err := r.decoder.PCMBuffer(buf)
			if err != nil {
				return fmt.Errorf("error seeking: %w", err)
			}
			if n == 0 {
				break // EOF
			}
			samplesRemaining -= n
		}
	}

	return nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsValid validates if the file is a valid WAV file.
func (r *WAVReader) IsValid() (bool, error) {
	if !r.isOpen {
		return false, errors.New("file not open")
	}

	// Check if it's a valid WAV file
	if !r.decoder.IsValidFile() {
		return false, nil
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
