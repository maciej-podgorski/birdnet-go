package buffer

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/smallnest/ringbuffer"
	"github.com/tphakala/birdnet-go/internal/conf"
)

// BufferManager handles the management of analysis and capture buffers
type BufferManager struct {
	// Analysis buffer management
	analysisBuffers        map[string]*ringbuffer.RingBuffer
	prevAnalysisData       map[string][]byte
	analysisWarningCounter map[string]int
	analysisMutex          sync.RWMutex

	// Capture buffer management
	captureBuffers map[string]*CaptureBuffer
	captureMutex   sync.RWMutex
}

// NewBufferManager creates a new buffer manager
func NewBufferManager() *BufferManager {
	return &BufferManager{
		analysisBuffers:        make(map[string]*ringbuffer.RingBuffer),
		prevAnalysisData:       make(map[string][]byte),
		analysisWarningCounter: make(map[string]int),
		captureBuffers:         make(map[string]*CaptureBuffer),
	}
}

// AllocateAnalysisBuffer initializes a ring buffer for a single audio source
func (bm *BufferManager) AllocateAnalysisBuffer(capacity int, source string) error {
	// Validate inputs
	if capacity <= 0 {
		return fmt.Errorf("invalid capacity: %d, must be greater than 0", capacity)
	}
	if source == "" {
		return fmt.Errorf("empty source name provided")
	}

	settings := conf.Setting()

	// Set overlapSize based on user setting in seconds
	overlapSize := int(settings.BirdNET.Overlap * float64(conf.SampleRate) * float64(conf.BitDepth/8))
	_ = conf.BufferSize - overlapSize // Calculate but not used directly in this method

	// Initialize the analysis ring buffer
	ab := ringbuffer.New(capacity)
	if ab == nil {
		return fmt.Errorf("failed to allocate ring buffer for source: %s", source)
	}

	// Update manager state safely
	bm.analysisMutex.Lock()
	defer bm.analysisMutex.Unlock()

	// Check if buffer already exists
	if _, exists := bm.analysisBuffers[source]; exists {
		ab.Reset() // Clean up the new buffer since we won't use it
		return fmt.Errorf("ring buffer already exists for source: %s", source)
	}

	bm.analysisBuffers[source] = ab
	bm.prevAnalysisData[source] = nil
	bm.analysisWarningCounter[source] = 0

	return nil
}

// RemoveAnalysisBuffer safely removes and cleans up a ring buffer for a single source
func (bm *BufferManager) RemoveAnalysisBuffer(source string) error {
	bm.analysisMutex.Lock()
	defer bm.analysisMutex.Unlock()

	ab, exists := bm.analysisBuffers[source]
	if !exists {
		return fmt.Errorf("no ring buffer found for source: %s", source)
	}

	// Clean up the buffer
	ab.Reset()

	// Remove from all maps
	delete(bm.analysisBuffers, source)
	delete(bm.prevAnalysisData, source)
	delete(bm.analysisWarningCounter, source)

	return nil
}

// AllocateCaptureBuffer initializes a capture buffer for a single source
func (bm *BufferManager) AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error {
	// Validate inputs
	if durationSeconds <= 0 {
		return fmt.Errorf("invalid duration: %d, must be greater than 0", durationSeconds)
	}
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate: %d, must be greater than 0", sampleRate)
	}
	if bytesPerSample <= 0 {
		return fmt.Errorf("invalid bytes per sample: %d, must be greater than 0", bytesPerSample)
	}
	if source == "" {
		return fmt.Errorf("empty source name provided")
	}

	// Calculate buffer size in bytes
	bufferSize := durationSeconds * sampleRate * bytesPerSample

	// Create a new CaptureBuffer
	cb := &CaptureBuffer{
		data:           make([]byte, bufferSize),
		sampleRate:     uint32(sampleRate),
		bytesPerSample: bytesPerSample,
		bufferSize:     bufferSize,
		bufferDuration: time.Duration(durationSeconds) * time.Second,
		startTime:      time.Now(),
		initialized:    true,
	}

	// Update manager state safely
	bm.captureMutex.Lock()
	defer bm.captureMutex.Unlock()

	// Check if buffer already exists
	if _, exists := bm.captureBuffers[source]; exists {
		return fmt.Errorf("capture buffer already exists for source: %s", source)
	}

	bm.captureBuffers[source] = cb

	return nil
}

// RemoveCaptureBuffer safely removes a capture buffer for a single source
func (bm *BufferManager) RemoveCaptureBuffer(source string) error {
	bm.captureMutex.Lock()
	defer bm.captureMutex.Unlock()

	_, exists := bm.captureBuffers[source]
	if !exists {
		return fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Remove the buffer entry
	delete(bm.captureBuffers, source)

	return nil
}

// WriteToAnalysisBuffer writes audio data into the ring buffer for a given stream
func (bm *BufferManager) WriteToAnalysisBuffer(stream string, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data provided")
	}

	// First get the buffer with a read lock
	bm.analysisMutex.RLock()
	ab, exists := bm.analysisBuffers[stream]
	bm.analysisMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no analysis buffer found for stream: %s", stream)
	}

	// Get buffer capacity information (safe since we're not modifying it)
	capacity := ab.Capacity()
	if capacity == 0 {
		return fmt.Errorf("analysis buffer for stream %s has zero capacity", stream)
	}

	// Check buffer capacity and update warning counter if needed
	const warningCapacityThreshold = 0.9 // 90% full
	capacityUsed := float64(ab.Length()) / float64(capacity)

	if capacityUsed > warningCapacityThreshold {
		// Use a separate lock for the warning counter to minimize lock contention
		bm.analysisMutex.Lock()
		counter := bm.analysisWarningCounter[stream]
		counter++
		bm.analysisWarningCounter[stream] = counter
		bm.analysisMutex.Unlock()

		if counter%32 == 1 {
			log.Printf("⚠️ Analysis buffer for stream %s is %.2f%% full (used: %d/%d bytes)",
				stream, capacityUsed*100, ab.Length(), capacity)
		}
	}

	// Write data to the ring buffer with multiple retries on failure
	const maxRetries = 3
	const retryDelay = time.Millisecond * 10

	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		// Lock only for the write operation
		bm.analysisMutex.Lock()
		n, err := ab.Write(data)
		bm.analysisMutex.Unlock()

		if err == nil {
			if n < len(data) {
				log.Printf("⚠️ Only wrote %d of %d bytes to buffer for stream %s (capacity: %d, free: %d)",
					n, len(data), stream, capacity, ab.Free())

				// Partial write is still a success, but we should log it
				return nil
			}

			// Full write succeeded
			return nil
		}

		// Save the last error
		lastErr = err

		// Log detailed buffer state
		log.Printf("⚠️ Analysis buffer for stream %s has %d/%d bytes free (%d bytes used), tried to write %d bytes",
			stream, ab.Free(), capacity, ab.Length(), len(data))

		if errors.Is(err, ringbuffer.ErrIsFull) {
			log.Printf("⚠️ Analysis buffer for stream %s is full. Waiting before retry %d/%d", stream, retry+1, maxRetries)
		} else {
			log.Printf("❌ Unexpected error writing to analysis buffer for stream %s: %v", stream, err)
		}

		if retry < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	// If we've reached this point, we've failed all retries
	return fmt.Errorf("failed to write to analysis buffer for stream %s after %d attempts: %w",
		stream, maxRetries, lastErr)
}

// ReadFromAnalysisBuffer reads a sliding chunk of audio data from the ring buffer for a given stream
func (bm *BufferManager) ReadFromAnalysisBuffer(stream string) ([]byte, error) {
	// Single lock for the entire read operation to ensure consistency
	bm.analysisMutex.Lock()
	defer bm.analysisMutex.Unlock()

	// Get the ring buffer for the given stream
	ab, exists := bm.analysisBuffers[stream]
	if !exists {
		return nil, fmt.Errorf("no analysis buffer found for stream: %s", stream)
	}

	// Calculate read size based on user setting (must be recalculated here to handle setting changes)
	settings := conf.Setting()
	overlapSize := int(settings.BirdNET.Overlap * float64(conf.SampleRate) * float64(conf.BitDepth/8))
	readSize := conf.BufferSize - overlapSize

	// Calculate the number of bytes written to the buffer
	bytesWritten := ab.Length()
	if bytesWritten < readSize {
		return nil, nil
	}

	// Create a slice to hold the data we're going to read
	data := make([]byte, readSize)
	// Read data from the ring buffer
	bytesRead, err := ab.Read(data)
	if err != nil {
		return nil, fmt.Errorf("error reading %d bytes from analysis buffer for stream: %s", bytesRead, stream)
	}

	// Join with previous data to ensure we're processing chunkSize bytes
	var fullData []byte
	bm.prevAnalysisData[stream] = append(bm.prevAnalysisData[stream], data...)
	fullData = bm.prevAnalysisData[stream]
	if len(fullData) >= conf.BufferSize {
		// Update prevData for the next iteration
		bm.prevAnalysisData[stream] = fullData[readSize:]
		fullData = fullData[:conf.BufferSize]
	} else {
		// If there isn't enough data even after appending, update prevData and return nil
		bm.prevAnalysisData[stream] = fullData
		return nil, nil
	}

	return fullData, nil
}

// WriteToCaptureBuffer adds PCM audio data to the buffer for a given source
func (bm *BufferManager) WriteToCaptureBuffer(source string, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data provided")
	}

	// First get the buffer with a read lock
	bm.captureMutex.RLock()
	cb, exists := bm.captureBuffers[source]
	bm.captureMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Write to the buffer with proper locking
	bm.captureMutex.Lock()
	defer bm.captureMutex.Unlock()

	if _, err := cb.Write(data); err != nil {
		return fmt.Errorf("failed to write to capture buffer: %w", err)
	}
	return nil
}

// ReadSegmentFromCaptureBuffer extracts a segment of audio data from the buffer for a given source
func (bm *BufferManager) ReadSegmentFromCaptureBuffer(source string, requestedStartTime time.Time, duration int) ([]byte, error) {
	// First check if the buffer exists with a read lock
	bm.captureMutex.RLock()
	cb, exists := bm.captureBuffers[source]
	bm.captureMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Read from the buffer with proper locking
	bm.captureMutex.Lock()
	defer bm.captureMutex.Unlock()

	return cb.ReadSegment(requestedStartTime, duration)
}

// CleanupAllBuffers removes all analysis and capture buffers
func (bm *BufferManager) CleanupAllBuffers() {
	// Clean up analysis buffers
	bm.analysisMutex.Lock()
	for sourceID, buffer := range bm.analysisBuffers {
		buffer.Reset()
		delete(bm.analysisBuffers, sourceID)
		delete(bm.prevAnalysisData, sourceID)
		delete(bm.analysisWarningCounter, sourceID)
	}
	bm.analysisMutex.Unlock()

	// Clean up capture buffers
	bm.captureMutex.Lock()
	for sourceID := range bm.captureBuffers {
		delete(bm.captureBuffers, sourceID)
	}
	bm.captureMutex.Unlock()
}

// HasAnalysisBuffer checks if an analysis buffer exists for a source
func (bm *BufferManager) HasAnalysisBuffer(source string) bool {
	bm.analysisMutex.RLock()
	defer bm.analysisMutex.RUnlock()
	_, exists := bm.analysisBuffers[source]
	return exists
}

// HasCaptureBuffer checks if a capture buffer exists for a source
func (bm *BufferManager) HasCaptureBuffer(source string) bool {
	bm.captureMutex.RLock()
	defer bm.captureMutex.RUnlock()
	_, exists := bm.captureBuffers[source]
	return exists
}
