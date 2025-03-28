package buffer

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// BufferManager handles the management of analysis and capture buffers
type BufferManager struct {
	// Analysis buffer management
	analysisBuffers map[string]AnalysisBufferInterface
	analysisMutex   sync.RWMutex

	// Capture buffer management
	captureBuffers map[string]CaptureBufferInterface
	captureMutex   sync.RWMutex

	// Dependencies
	logger        Logger
	timeProvider  TimeProvider
	config        *BufferConfig
	bufferFactory BufferFactoryInterface
}

// NewBufferManager creates a new buffer manager with default dependencies
func NewBufferManager() *BufferManager {
	factory := NewBufferFactory()
	return NewBufferManagerWithDeps(
		factory.logger,
		factory.timeProvider,
		factory.config,
		factory,
	)
}

// NewBufferManagerWithDeps creates a new buffer manager with custom dependencies
func NewBufferManagerWithDeps(
	logger Logger,
	timeProvider TimeProvider,
	config *BufferConfig,
	bufferFactory BufferFactoryInterface,
) *BufferManager {
	return &BufferManager{
		analysisBuffers: make(map[string]AnalysisBufferInterface),
		captureBuffers:  make(map[string]CaptureBufferInterface),
		logger:          logger,
		timeProvider:    timeProvider,
		config:          config,
		bufferFactory:   bufferFactory,
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

	// Calculate buffer duration based on capacity
	sampleRate := uint32(bm.config.SampleRate)
	channels := uint32(1) // Assuming mono for now
	bytesPerSample := 2   // Assuming 16-bit samples

	// Calculate duration from capacity, sample rate, channels, and bytes per sample
	bytesPerSecond := int(sampleRate * channels * uint32(bytesPerSample))
	if bytesPerSecond <= 0 {
		return fmt.Errorf("invalid bytes per second: %d", bytesPerSecond)
	}

	durationSeconds := float64(capacity) / float64(bytesPerSecond)
	duration := time.Duration(durationSeconds * float64(time.Second))

	// Log what we're doing to help debug
	bm.logger.Info("Creating analysis buffer for source %s with capacity %d bytes (%.2f seconds)",
		source, capacity, durationSeconds)

	// Create a new analysis buffer with the calculated duration
	ab := bm.bufferFactory.CreateAnalysisBuffer(
		sampleRate,
		channels,
		duration, // Use the duration we calculated from capacity
	)

	if ab == nil {
		return fmt.Errorf("failed to create analysis buffer for source: %s", source)
	}

	// Update manager state safely
	bm.analysisMutex.Lock()
	defer bm.analysisMutex.Unlock()

	// Check if buffer already exists
	if _, exists := bm.analysisBuffers[source]; exists {
		return fmt.Errorf("analysis buffer already exists for source: %s", source)
	}

	bm.analysisBuffers[source] = ab
	return nil
}

// RemoveAnalysisBuffer safely removes and cleans up a ring buffer for a single source
func (bm *BufferManager) RemoveAnalysisBuffer(source string) error {
	bm.analysisMutex.Lock()
	defer bm.analysisMutex.Unlock()

	ab, exists := bm.analysisBuffers[source]
	if !exists {
		return fmt.Errorf("no analysis buffer found for source: %s", source)
	}

	// Clean up the buffer
	if err := ab.Reset(); err != nil {
		return fmt.Errorf("error resetting analysis buffer: %w", err)
	}

	// Remove from the map
	delete(bm.analysisBuffers, source)
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

	// Create a new CaptureBuffer using factory
	cb := bm.bufferFactory.CreateCaptureBuffer(
		uint32(sampleRate),
		1, // Assuming mono for simplicity
		time.Duration(durationSeconds)*time.Second,
	)

	if cb == nil {
		return fmt.Errorf("failed to create capture buffer for source: %s", source)
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

	cb, exists := bm.captureBuffers[source]
	if !exists {
		return fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Clean up the buffer
	if err := cb.Reset(); err != nil {
		return fmt.Errorf("error resetting capture buffer: %w", err)
	}

	// Remove the buffer entry
	delete(bm.captureBuffers, source)
	return nil
}

// WriteToAnalysisBuffer writes audio data into the analysis buffer for a given stream
func (bm *BufferManager) WriteToAnalysisBuffer(stream string, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data provided")
	}

	// Get the buffer with a read lock first
	bm.analysisMutex.RLock()
	ab, exists := bm.analysisBuffers[stream]
	bm.analysisMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no analysis buffer found for stream: %s", stream)
	}

	// Delegate to the buffer's Write method
	_, err := ab.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to analysis buffer: %w", err)
	}

	return nil
}

// ReadFromAnalysisBuffer reads a sliding chunk of audio data from the buffer for a given stream
func (bm *BufferManager) ReadFromAnalysisBuffer(stream string, optionalConfig *BufferConfig) ([]byte, error) {
	// Get the buffer with a read lock
	bm.analysisMutex.RLock()
	ab, exists := bm.analysisBuffers[stream]
	bm.analysisMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no analysis buffer found for stream: %s", stream)
	}

	// Check if the buffer is ready for analysis
	if !ab.ReadyForAnalysis() {
		return nil, nil // Not enough data yet
	}

	// Use provided config or fall back to default
	config := bm.config
	if optionalConfig != nil {
		config = optionalConfig
	}

	// Calculate the buffer size we need based on config
	bufferSize := config.BufferSize
	data := make([]byte, bufferSize)

	// Delegate to the buffer's Read method
	n, err := ab.Read(data)
	if err != nil {
		return nil, fmt.Errorf("error reading from analysis buffer: %w", err)
	}

	if n == 0 {
		return nil, nil // Not enough data yet
	}

	return data[:n], nil
}

// WriteToCaptureBuffer adds PCM audio data to the buffer for a given source
func (bm *BufferManager) WriteToCaptureBuffer(source string, data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data provided")
	}

	// Get the buffer with a read lock
	bm.captureMutex.RLock()
	cb, exists := bm.captureBuffers[source]
	bm.captureMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Delegate to the buffer's Write method
	_, err := cb.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to capture buffer: %w", err)
	}

	return nil
}

// ReadSegmentFromCaptureBuffer extracts a segment of audio data from the buffer for a given source
func (bm *BufferManager) ReadSegmentFromCaptureBuffer(source string, requestedStartTime time.Time, duration int) ([]byte, error) {
	// Get the buffer with a read lock
	bm.captureMutex.RLock()
	cb, exists := bm.captureBuffers[source]
	bm.captureMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no capture buffer found for source: %s", source)
	}

	// Delegate to the buffer's ReadSegment method
	return cb.ReadSegment(requestedStartTime, duration)
}

// CleanupAllBuffers removes all analysis and capture buffers
func (bm *BufferManager) CleanupAllBuffers() {
	// Clean up analysis buffers
	bm.analysisMutex.Lock()
	for sourceID, buffer := range bm.analysisBuffers {
		err := buffer.Reset()
		if err != nil {
			bm.logger.Error("error resetting analysis buffer: %w", err)
		}
		delete(bm.analysisBuffers, sourceID)
	}
	bm.analysisMutex.Unlock()

	// Clean up capture buffers
	bm.captureMutex.Lock()
	for sourceID, buffer := range bm.captureBuffers {
		err := buffer.Reset()
		if err != nil {
			bm.logger.Error("error resetting capture buffer: %w", err)
		}
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

// GetAnalysisBufferCapacity returns the capacity of the analysis buffer for a given source
// This is useful for debugging buffer sizing issues
func (bm *BufferManager) GetAnalysisBufferCapacity(source string) (int, error) {
	bm.analysisMutex.RLock()
	defer bm.analysisMutex.RUnlock()

	ab, exists := bm.analysisBuffers[source]
	if !exists {
		return 0, fmt.Errorf("no analysis buffer found for source: %s", source)
	}

	return ab.Capacity(), nil
}
