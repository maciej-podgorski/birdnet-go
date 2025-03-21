package buffer

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/smallnest/ringbuffer"
)

// AnalysisBuffer implements the AnalysisBuffer interface for audio analysis.
type AnalysisBuffer struct {
	buffer     *ringbuffer.RingBuffer
	prevData   []byte
	sampleRate uint32
	channels   uint32
	threshold  int

	warningCounter int
	mu             sync.RWMutex
}

// NewAnalysisBuffer creates a new analysis buffer.
func NewAnalysisBuffer(sampleRate, channels uint32, duration time.Duration) *AnalysisBuffer {
	// Calculate buffer size based on duration, sample rate, channels and 2 bytes per sample
	bytesPerSecond := int(sampleRate * channels * 2)
	bufferSize := bytesPerSecond * int(duration.Seconds())

	// Analysis threshold is typically 75% of the buffer
	threshold := bufferSize * 3 / 4

	return &AnalysisBuffer{
		buffer:         ringbuffer.New(bufferSize),
		prevData:       nil,
		sampleRate:     sampleRate,
		channels:       channels,
		threshold:      threshold,
		warningCounter: 0,
	}
}

// Write writes audio data to the buffer.
func (ab *AnalysisBuffer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("empty data provided")
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	// Check buffer capacity
	capacity := ab.buffer.Capacity()
	if capacity == 0 {
		return 0, errors.New("buffer has zero capacity")
	}

	// Check if buffer is nearing capacity
	const warningCapacityThreshold = 0.9 // 90% full
	capacityUsed := float64(ab.buffer.Length()) / float64(capacity)

	if capacityUsed > warningCapacityThreshold {
		ab.warningCounter++
		// Only log every 32nd warning to avoid flooding logs
		if ab.warningCounter%32 == 1 {
			fmt.Printf("⚠️ Analysis buffer is %.2f%% full (used: %d/%d bytes)\n",
				capacityUsed*100, ab.buffer.Length(), capacity)
		}
	}

	// Write data to the ring buffer with retry logic
	const maxRetries = 3
	const retryDelay = time.Millisecond * 10

	var lastErr error
	var n int

	for retry := 0; retry < maxRetries; retry++ {
		n, lastErr = ab.buffer.Write(data)

		if lastErr == nil {
			if n < len(data) {
				fmt.Printf("⚠️ Only wrote %d of %d bytes to buffer (capacity: %d, free: %d)\n",
					n, len(data), capacity, ab.buffer.Free())

				// Partial write is still a success
				return n, nil
			}

			// Full write succeeded
			return n, nil
		}

		// Log detailed buffer state on error
		fmt.Printf("⚠️ Buffer has %d/%d bytes free (%d bytes used), tried to write %d bytes\n",
			ab.buffer.Free(), capacity, ab.buffer.Length(), len(data))

		if errors.Is(lastErr, ringbuffer.ErrIsFull) {
			fmt.Printf("⚠️ Buffer is full. Waiting before retry %d/%d\n", retry+1, maxRetries)
		} else {
			fmt.Printf("❌ Unexpected error writing to buffer: %v\n", lastErr)
		}

		if retry < maxRetries-1 {
			// Release lock during sleep
			ab.mu.Unlock()
			time.Sleep(retryDelay)
			ab.mu.Lock()
		}
	}

	// Failed all retries
	return 0, fmt.Errorf("failed to write to buffer after %d attempts: %w",
		maxRetries, lastErr)
}

// Read reads audio data from the buffer with sliding window approach.
func (ab *AnalysisBuffer) Read(p []byte) (int, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// Calculate read size based on the overlap
	// TODO: FIXME we should calculate overlap based on settings.birdnet.overlap
	// TODO: FIXME we should also calculate the read size based on the overlap
	var overlapFraction = 0.25
	readSize := len(p) * (1 - int(overlapFraction))

	// Calculate the number of bytes in the buffer
	bytesAvailable := ab.buffer.Length()
	if bytesAvailable < readSize {
		return 0, nil // Not enough data yet
	}

	// Create a slice to read data into
	data := make([]byte, readSize)

	// Read data from the ring buffer
	bytesRead, err := ab.buffer.Read(data)
	if err != nil {
		return 0, fmt.Errorf("error reading %d bytes from buffer: %w", bytesRead, err)
	}

	// Join with previous data to ensure we're processing the right amount of bytes
	if ab.prevData == nil {
		ab.prevData = data
		return 0, nil // First read, need more data
	}

	ab.prevData = append(ab.prevData, data...)

	if len(ab.prevData) >= len(p) {
		// Copy the data into the output buffer
		n := copy(p, ab.prevData[:len(p)])

		// Update prevData for the next iteration
		ab.prevData = ab.prevData[readSize:]

		return n, nil
	}

	// Not enough data yet
	return 0, nil
}

// ReadyForAnalysis returns whether the buffer is ready for analysis.
func (ab *AnalysisBuffer) ReadyForAnalysis() bool {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	// We're ready for analysis if we have data in the buffer
	// and it exceeds the analysis threshold
	return ab.buffer.Length() >= ab.threshold
}

// Reset resets the buffer.
func (ab *AnalysisBuffer) Reset() error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.buffer.Reset()
	ab.prevData = nil
	ab.warningCounter = 0

	return nil
}

// SampleRate returns the sample rate of the buffer.
func (ab *AnalysisBuffer) SampleRate() uint32 {
	return ab.sampleRate
}

// Channels returns the number of channels in the buffer.
func (ab *AnalysisBuffer) Channels() uint32 {
	return ab.channels
}
