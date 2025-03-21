package buffer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufferFactory_NewBufferFactory(t *testing.T) {
	// Create the factory
	factory := NewBufferFactory()

	// Assert it has all dependencies
	assert.NotNil(t, factory.logger)
	assert.NotNil(t, factory.timeProvider)
	assert.NotNil(t, factory.config)
	assert.NotNil(t, factory.ringBufferFactory)

	// Test it creates a non-nil buffer
	ringBuffer := factory.ringBufferFactory(1000)
	assert.NotNil(t, ringBuffer)
}

func TestBufferFactory_CreateAnalysisBuffer(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)
	mockConfig := NewDefaultBufferConfig()
	mockRingBuffer := new(MockRingBufferInterface)

	// Create a mock ring buffer factory
	mockRingBufferFactory := func(size int) RingBufferInterface {
		return mockRingBuffer
	}

	// Setup expectations
	mockRingBuffer.On("Capacity").Return(1000).Maybe()

	// Create the factory with mocks
	factory := NewBufferFactoryWithDeps(
		mockLogger,
		mockTimeProvider,
		mockConfig,
		mockRingBufferFactory,
	)

	// Execute the method
	buffer := factory.CreateAnalysisBuffer(48000, 2, 3*time.Second)

	// Assert results
	assert.NotNil(t, buffer)

	// Cast to concrete type to verify inner fields
	analysisBuffer, ok := buffer.(*AnalysisBuffer)
	if assert.True(t, ok, "Should be *AnalysisBuffer type") {
		assert.Equal(t, uint32(48000), analysisBuffer.sampleRate)
		assert.Equal(t, uint32(2), analysisBuffer.channels)
		assert.Equal(t, mockRingBuffer, analysisBuffer.buffer)
		assert.Equal(t, mockLogger, analysisBuffer.logger)
		assert.Equal(t, mockTimeProvider, analysisBuffer.timeProvider)
	}
}

func TestBufferFactory_CreateCaptureBuffer(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)
	mockConfig := NewDefaultBufferConfig()

	// Create a mock ring buffer factory
	mockRingBufferFactory := func(size int) RingBufferInterface {
		return new(MockRingBufferInterface)
	}

	// Create the factory with mocks
	factory := NewBufferFactoryWithDeps(
		mockLogger,
		mockTimeProvider,
		mockConfig,
		mockRingBufferFactory,
	)

	// Execute the method
	buffer := factory.CreateCaptureBuffer(44100, 1, 5*time.Second)

	// Assert results
	assert.NotNil(t, buffer)

	// Cast to concrete type to verify inner fields
	captureBuffer, ok := buffer.(*CaptureBuffer)
	if assert.True(t, ok, "Should be *CaptureBuffer type") {
		assert.Equal(t, uint32(44100), captureBuffer.sampleRate)
		assert.Equal(t, uint32(1), captureBuffer.channels)
		assert.Equal(t, 5*time.Second, captureBuffer.bufferDuration)
		assert.Equal(t, mockLogger, captureBuffer.logger)
		assert.Equal(t, mockTimeProvider, captureBuffer.timeProvider)
	}
}

func TestBufferFactory_CreateBufferManager(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)
	mockConfig := NewDefaultBufferConfig()

	// Create a mock ring buffer factory
	mockRingBufferFactory := func(size int) RingBufferInterface {
		return new(MockRingBufferInterface)
	}

	// Create the factory with mocks
	factory := NewBufferFactoryWithDeps(
		mockLogger,
		mockTimeProvider,
		mockConfig,
		mockRingBufferFactory,
	)

	// Execute the method
	manager := factory.CreateBufferManager()

	// Assert results
	assert.NotNil(t, manager)

	// Cast to concrete type to verify inner fields
	bufferManager, ok := manager.(*BufferManager)
	if assert.True(t, ok, "Should be *BufferManager type") {
		assert.Equal(t, mockLogger, bufferManager.logger)
		assert.Equal(t, mockTimeProvider, bufferManager.timeProvider)
		assert.Equal(t, mockConfig, bufferManager.config)
		assert.Equal(t, factory, bufferManager.bufferFactory)
	}
}
