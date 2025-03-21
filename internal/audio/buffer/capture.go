package buffer

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// CaptureBuffer represents a circular buffer for storing PCM audio data, with timestamp tracking.
type CaptureBuffer struct {
	data           []byte
	writeIndex     int
	sampleRate     uint32
	channels       uint32
	bytesPerSample int
	bufferSize     int
	bufferDuration time.Duration
	startTime      time.Time
	initialized    bool
	mu             sync.Mutex
}

// NewCaptureBuffer creates a new capture buffer.
func NewCaptureBuffer(sampleRate, channels uint32, duration time.Duration) *CaptureBuffer {
	// Calculate buffer parameters
	bytesPerSample := 2 // Assuming 16-bit samples
	bytesPerSecond := int(sampleRate*channels) * bytesPerSample
	bufferSize := bytesPerSecond * int(duration.Seconds())

	// Round up buffer size to aligned boundary
	alignedBufferSize := ((bufferSize + 2047) / 2048) * 2048

	return &CaptureBuffer{
		data:           make([]byte, alignedBufferSize),
		writeIndex:     0,
		sampleRate:     sampleRate,
		channels:       channels,
		bytesPerSample: bytesPerSample,
		bufferSize:     alignedBufferSize,
		bufferDuration: duration,
		initialized:    false,
	}
}

// Write adds PCM audio data to the buffer.
func (cb *CaptureBuffer) Write(data []byte) (bytesWritten int, err error) {
	if len(data) == 0 {
		return 0, errors.New("empty data provided")
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.initialized {
		// Initialize the buffer's start time when first data arrives
		cb.startTime = time.Now()
		cb.initialized = true
	}

	// Store the current write index to detect buffer wrap-around
	prevWriteIndex := cb.writeIndex

	// Copy data into the buffer
	bytesWritten = copy(cb.data[cb.writeIndex:], data)

	// Handle potential wrap-around
	if bytesWritten < len(data) {
		// Copy remaining data to the beginning of the buffer
		bytesWritten += copy(cb.data, data[bytesWritten:])
	}

	// Update write index with wrap-around
	cb.writeIndex = (cb.writeIndex + bytesWritten) % cb.bufferSize

	// If buffer has wrapped around, adjust start time to maintain accurate timestamps
	if cb.writeIndex <= prevWriteIndex && bytesWritten > 0 {
		cb.startTime = time.Now().Add(-cb.bufferDuration)
	}

	return bytesWritten, nil
}

// Read reads audio data from the buffer.
func (cb *CaptureBuffer) Read(p []byte) (bytesRead int, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.initialized {
		return 0, errors.New("buffer not initialized")
	}

	// Since this is a circular buffer, we need to handle wrap-around
	// For simplicity, we'll just read from the current write position minus len(p)

	readIndex := (cb.writeIndex - len(p)) % cb.bufferSize
	if readIndex < 0 {
		readIndex += cb.bufferSize
	}

	// If readIndex is now after writeIndex, the buffer hasn't been filled yet
	if cb.writeIndex > 0 && readIndex > cb.writeIndex {
		return 0, errors.New("not enough data in buffer")
	}

	// Handle wrap-around case
	if readIndex+len(p) > cb.bufferSize {
		// Read from readIndex to end of buffer
		bytesRead = copy(p, cb.data[readIndex:])
		// Read remaining bytes from beginning of buffer
		bytesRead += copy(p[bytesRead:], cb.data[:len(p)-bytesRead])
	} else {
		// Simple case - read contiguous region
		bytesRead = copy(p, cb.data[readIndex:readIndex+len(p)])
	}

	return bytesRead, nil
}

// ReadSegment extracts a segment of audio data based on start time and duration.
func (cb *CaptureBuffer) ReadSegment(requestedStartTime time.Time, durationSeconds int) (segment []byte, err error) {
	requestedEndTime := requestedStartTime.Add(time.Duration(durationSeconds) * time.Second)

	// Try to read until we have data or timeout
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		cb.mu.Lock()

		if !cb.initialized {
			cb.mu.Unlock()
			return nil, errors.New("buffer not initialized")
		}

		// Calculate time offsets relative to buffer start time
		startOffset := requestedStartTime.Sub(cb.startTime)
		endOffset := requestedEndTime.Sub(cb.startTime)

		// Convert time offsets to buffer indices
		bytesPerSecond := int(cb.sampleRate*cb.channels) * cb.bytesPerSample
		startIndex := int(startOffset.Seconds() * float64(bytesPerSecond))
		endIndex := int(endOffset.Seconds() * float64(bytesPerSecond))

		// Handle negative start index (requested time before buffer start)
		if startOffset < 0 {
			cb.mu.Unlock()
			return nil, fmt.Errorf("requested start time is before buffer start time")
		}

		// Handle invalid end index
		if endOffset < 0 || endOffset <= startOffset {
			cb.mu.Unlock()
			return nil, fmt.Errorf("invalid end time")
		}

		// Ensure indices are within buffer bounds
		startIndex %= cb.bufferSize
		endIndex %= cb.bufferSize

		// Check if we have the requested data timeframe
		currentTime := time.Now()
		if currentTime.After(requestedEndTime) {
			// Extract the segment
			var segment []byte

			var segmentSize int
			if startIndex < endIndex {
				// Simple case - continuous segment
				segmentSize = endIndex - startIndex
				segment = make([]byte, segmentSize)
				copy(segment, cb.data[startIndex:endIndex])
			} else {
				// Wrap-around case
				segmentSize = (cb.bufferSize - startIndex) + endIndex
				segment = make([]byte, segmentSize)
				copy(segment, cb.data[startIndex:])
				copy(segment[cb.bufferSize-startIndex:], cb.data[:endIndex])
			}

			cb.mu.Unlock()
			return segment, nil
		}

		cb.mu.Unlock()

		// Wait for more data
		time.Sleep(time.Second)
	}

	return nil, errors.New("timeout waiting for data")
}

// Reset resets the buffer.
func (cb *CaptureBuffer) Reset() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.writeIndex = 0
	cb.initialized = false

	return nil
}

// SampleRate returns the sample rate of the buffer.
func (cb *CaptureBuffer) SampleRate() uint32 {
	return cb.sampleRate
}

// Channels returns the number of channels in the buffer.
func (cb *CaptureBuffer) Channels() uint32 {
	return cb.channels
}
