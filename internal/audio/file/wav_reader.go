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

	if r.debug {
		fmt.Printf("WAV Info: %+v\n", r.info)
	}

	return nil
}

// Close closes the file and releases resources.
func (r *WAVReader) Close() error {
	r.isOpen = false
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

// ReadChunk reads the next chunk of audio data.
func (r *WAVReader) ReadChunk(chunkDuration float64, overlap float64) (Chunk, error) {
	if !r.isOpen {
		return Chunk{}, errors.New("file not open")
	}

	// Calculate number of samples in this chunk
	chunkSamples := int(chunkDuration * float64(r.info.SampleRate))

	if r.debug {
		fmt.Printf("DEBUG WAV Reader: Reading chunk of %d samples, current position %d, total samples %d\n",
			chunkSamples, r.lastPos, r.info.TotalSamples)
	}

	// Check if we've reached the end of the file
	if r.lastPos >= r.info.TotalSamples {
		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Already at end of file, position %d >= total samples %d\n",
				r.lastPos, r.info.TotalSamples)
		}
		return Chunk{}, io.EOF
	}

	// Seek to the current position before reading
	if err := r.Seek(r.lastPos); err != nil {
		return Chunk{}, fmt.Errorf("error seeking to position %d: %w", r.lastPos, err)
	}

	// Buffer to hold the PCM data
	buf := &audio.IntBuffer{
		Data:   make([]int, chunkSamples),
		Format: &audio.Format{SampleRate: r.info.SampleRate, NumChannels: r.info.NumChannels},
	}

	// Read samples from the decoder
	n, err := r.decoder.PCMBuffer(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return Chunk{}, fmt.Errorf("error reading WAV data: %w", err)
	}

	// If we read 0 samples and got EOF, we're done
	if n == 0 && (err == io.EOF || err == nil) {
		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Reached EOF at position %d of %d total samples\n",
				r.lastPos, r.info.TotalSamples)
		}
		return Chunk{}, io.EOF
	}

	// Get the divisor for normalizing samples
	divisor, err := getAudioDivisor(r.info.BitDepth)
	if err != nil {
		return Chunk{}, err
	}

	// Convert int samples to float32
	floatData := make([]float32, n)
	for i := 0; i < n; i++ {
		floatData[i] = float32(buf.Data[i]) / divisor
	}

	// If we didn't read a full chunk, pad with zeros
	if n < chunkSamples {
		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Padding chunk with %d zeros (read %d of %d samples)\n",
				chunkSamples-n, n, chunkSamples)
		}
		floatData = append(floatData, make([]float32, chunkSamples-n)...)
	}

	// Calculate start time relative to the beginning of the file
	sampleOffset := r.lastPos
	startTime := time.Duration(float64(sampleOffset) / float64(r.info.SampleRate) * float64(time.Second))

	// Update the position for the next read
	// Calculate step size in samples based on chunk duration and overlap in seconds
	stepSamples := int((chunkDuration - overlap) * float64(r.info.SampleRate))
	r.lastPos += stepSamples

	if r.debug {
		fmt.Printf("DEBUG WAV Reader: Advanced position by %d samples to %d, overlap %.2f seconds\n",
			stepSamples, r.lastPos, overlap)
	}

	// Check if next position would be beyond the file
	if r.lastPos >= r.info.TotalSamples {
		if r.debug {
			fmt.Printf("DEBUG WAV Reader: Next position %d would exceed total samples %d\n",
				r.lastPos, r.info.TotalSamples)
		}
	}

	return Chunk{
		Data:         floatData,
		StartTime:    time.Time{}.Add(startTime), // Using zero time as base
		SampleOffset: sampleOffset,
		NumSamples:   n,
	}, nil
}

// ProcessFile processes the entire file in chunks.
func (r *WAVReader) ProcessFile(ctx context.Context, chunkDuration float64, overlap float64, processor ChunkProcessor) error {
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
func (r *WAVReader) Seek(sampleOffset int) error {
	if !r.isOpen {
		return errors.New("file not open")
	}

	// WAV decoder doesn't have a direct seek method, so we need to reopen the file
	// and read up to the desired position
	filePath := r.info.Path
	r.Close()

	if err := r.Open(filePath); err != nil {
		return err
	}

	// Skip samples until we reach the desired offset
	// This is inefficient but works as a basic implementation
	if sampleOffset > 0 {
		chunkSize := 4096 // Read in 4K chunks for efficiency
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

	r.lastPos = sampleOffset
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
