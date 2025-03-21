package buffer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBufferManager_AllocateAnalysisBuffer(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)
	mockConfig := NewDefaultBufferConfig()
	mockFactory := new(MockBufferFactory)
	mockBuffer := new(MockAnalysisBufferInterface)

	// Setup expectations
	mockFactory.On("CreateAnalysisBuffer",
		uint32(mockConfig.SampleRate),
		uint32(1),
		mockConfig.DefaultAnalysisDuration,
	).Return(mockBuffer)

	// Create buffer manager
	manager := &BufferManager{
		analysisBuffers: make(map[string]AnalysisBufferInterface),
		captureBuffers:  make(map[string]CaptureBufferInterface),
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
		config:          mockConfig,
		bufferFactory:   mockFactory,
	}

	// Execute the method
	err := manager.AllocateAnalysisBuffer(1000, "test-source")

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, mockBuffer, manager.analysisBuffers["test-source"])

	// Verify expectations
	mockFactory.AssertExpectations(t)
}

func TestBufferManager_AllocateAnalysisBuffer_InvalidCapacity(t *testing.T) {
	// Create buffer manager with minimal dependencies
	manager := &BufferManager{
		analysisBuffers: make(map[string]AnalysisBufferInterface),
		captureBuffers:  make(map[string]CaptureBufferInterface),
		logger:          new(MockLogger),
		timeProvider:    new(MockTimeProvider),
		config:          NewDefaultBufferConfig(),
		bufferFactory:   new(MockBufferFactory),
	}

	// Execute the method with invalid capacity
	err := manager.AllocateAnalysisBuffer(0, "test-source")

	// Assert results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid capacity")
}

func TestBufferManager_RemoveAnalysisBuffer(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockAnalysisBufferInterface)

	// Setup expectations
	mockBuffer.On("Reset").Return(nil)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"test-source": mockBuffer,
		},
		captureBuffers: make(map[string]CaptureBufferInterface),
		logger:         new(MockLogger),
		timeProvider:   new(MockTimeProvider),
		config:         NewDefaultBufferConfig(),
		bufferFactory:  new(MockBufferFactory),
	}

	// Execute the method
	err := manager.RemoveAnalysisBuffer("test-source")

	// Assert results
	assert.NoError(t, err)
	assert.Empty(t, manager.analysisBuffers)

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_WriteToAnalysisBuffer(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockAnalysisBufferInterface)

	// Setup test data
	testData := []byte{1, 2, 3, 4, 5}

	// Setup expectations
	mockBuffer.On("Write", testData).Return(len(testData), nil)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"test-source": mockBuffer,
		},
		captureBuffers: make(map[string]CaptureBufferInterface),
		logger:         new(MockLogger),
		timeProvider:   new(MockTimeProvider),
		config:         NewDefaultBufferConfig(),
		bufferFactory:  new(MockBufferFactory),
	}

	// Execute the method
	err := manager.WriteToAnalysisBuffer("test-source", testData)

	// Assert results
	assert.NoError(t, err)

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_ReadFromAnalysisBuffer(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockAnalysisBufferInterface)
	mockConfig := NewDefaultBufferConfig()

	// Setup test data
	expectedData := []byte{1, 2, 3, 4, 5}
	outputBuf := make([]byte, mockConfig.BufferSize)
	copy(outputBuf, expectedData)

	// Setup expectations
	mockBuffer.On("ReadyForAnalysis").Return(true)
	mockBuffer.On("Read", mock.Anything).Run(func(args mock.Arguments) {
		// Copy our test data to the provided buffer
		buf := args.Get(0).([]byte)
		copy(buf, expectedData)
	}).Return(len(expectedData), nil)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"test-source": mockBuffer,
		},
		captureBuffers: make(map[string]CaptureBufferInterface),
		logger:         new(MockLogger),
		timeProvider:   new(MockTimeProvider),
		config:         mockConfig,
		bufferFactory:  new(MockBufferFactory),
	}

	// Execute the method
	data, err := manager.ReadFromAnalysisBuffer("test-source", nil)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, expectedData, data)

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_ReadFromAnalysisBuffer_NotReady(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockAnalysisBufferInterface)

	// Setup expectations
	mockBuffer.On("ReadyForAnalysis").Return(false)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"test-source": mockBuffer,
		},
		captureBuffers: make(map[string]CaptureBufferInterface),
		logger:         new(MockLogger),
		timeProvider:   new(MockTimeProvider),
		config:         NewDefaultBufferConfig(),
		bufferFactory:  new(MockBufferFactory),
	}

	// Execute the method
	data, err := manager.ReadFromAnalysisBuffer("test-source", nil)

	// Assert results
	assert.NoError(t, err)
	assert.Nil(t, data) // Should return nil when not ready

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_ReadFromAnalysisBuffer_CustomConfig(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockAnalysisBufferInterface)
	defaultConfig := NewDefaultBufferConfig()
	customConfig := &BufferConfig{
		BirdNETOverlap:          0.5,
		SampleRate:              44100,
		BitDepth:                24,
		BufferSize:              1000,
		DefaultAnalysisDuration: 2 * time.Second,
		DefaultCaptureDuration:  20 * time.Second,
	}

	// Setup test data
	expectedData := []byte{1, 2, 3, 4, 5}

	// Setup expectations
	mockBuffer.On("ReadyForAnalysis").Return(true)
	mockBuffer.On("Read", mock.Anything).Run(func(args mock.Arguments) {
		// Verify that buffer size is from custom config
		buf := args.Get(0).([]byte)
		assert.Equal(t, customConfig.BufferSize, len(buf))
		copy(buf, expectedData)
	}).Return(len(expectedData), nil)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"test-source": mockBuffer,
		},
		captureBuffers: make(map[string]CaptureBufferInterface),
		logger:         new(MockLogger),
		timeProvider:   new(MockTimeProvider),
		config:         defaultConfig,
		bufferFactory:  new(MockBufferFactory),
	}

	// Execute the method with custom config
	data, err := manager.ReadFromAnalysisBuffer("test-source", customConfig)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, expectedData, data)

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_AllocateCaptureBuffer(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)
	mockConfig := NewDefaultBufferConfig()
	mockFactory := new(MockBufferFactory)
	mockBuffer := new(MockCaptureBufferInterface)

	// Setup expectations
	mockFactory.On("CreateCaptureBuffer",
		uint32(48000), // Sample rate
		uint32(1),     // Channels
		3*time.Second, // Duration
	).Return(mockBuffer)

	// Create buffer manager
	manager := &BufferManager{
		analysisBuffers: make(map[string]AnalysisBufferInterface),
		captureBuffers:  make(map[string]CaptureBufferInterface),
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
		config:          mockConfig,
		bufferFactory:   mockFactory,
	}

	// Execute the method
	err := manager.AllocateCaptureBuffer(3, 48000, 2, "test-source")

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, mockBuffer, manager.captureBuffers["test-source"])

	// Verify expectations
	mockFactory.AssertExpectations(t)
}

func TestBufferManager_ReadSegmentFromCaptureBuffer(t *testing.T) {
	// Create mocks
	mockBuffer := new(MockCaptureBufferInterface)

	// Setup test data
	startTime := time.Now()
	duration := 3
	expectedData := []byte{1, 2, 3, 4, 5}

	// Setup expectations
	mockBuffer.On("ReadSegment", startTime, duration).Return(expectedData, nil)

	// Create buffer manager with existing buffer
	manager := &BufferManager{
		analysisBuffers: make(map[string]AnalysisBufferInterface),
		captureBuffers: map[string]CaptureBufferInterface{
			"test-source": mockBuffer,
		},
		logger:        new(MockLogger),
		timeProvider:  new(MockTimeProvider),
		config:        NewDefaultBufferConfig(),
		bufferFactory: new(MockBufferFactory),
	}

	// Execute the method
	data, err := manager.ReadSegmentFromCaptureBuffer("test-source", startTime, duration)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, expectedData, data)

	// Verify expectations
	mockBuffer.AssertExpectations(t)
}

func TestBufferManager_CleanupAllBuffers(t *testing.T) {
	// Create mocks
	mockAnalysisBuffer := new(MockAnalysisBufferInterface)
	mockCaptureBuffer := new(MockCaptureBufferInterface)
	mockLogger := new(MockLogger)

	// Setup expectations
	mockAnalysisBuffer.On("Reset").Return(nil)
	mockCaptureBuffer.On("Reset").Return(nil)

	// Create buffer manager with existing buffers
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"analysis-source": mockAnalysisBuffer,
		},
		captureBuffers: map[string]CaptureBufferInterface{
			"capture-source": mockCaptureBuffer,
		},
		logger:        mockLogger,
		timeProvider:  new(MockTimeProvider),
		config:        NewDefaultBufferConfig(),
		bufferFactory: new(MockBufferFactory),
	}

	// Execute the method
	manager.CleanupAllBuffers()

	// Assert results
	assert.Empty(t, manager.analysisBuffers)
	assert.Empty(t, manager.captureBuffers)

	// Verify expectations
	mockAnalysisBuffer.AssertExpectations(t)
	mockCaptureBuffer.AssertExpectations(t)
}

func TestBufferManager_HasBuffers(t *testing.T) {
	// Create mocks
	mockAnalysisBuffer := new(MockAnalysisBufferInterface)
	mockCaptureBuffer := new(MockCaptureBufferInterface)

	// Create buffer manager with existing buffers
	manager := &BufferManager{
		analysisBuffers: map[string]AnalysisBufferInterface{
			"analysis-source": mockAnalysisBuffer,
		},
		captureBuffers: map[string]CaptureBufferInterface{
			"capture-source": mockCaptureBuffer,
		},
		logger:        new(MockLogger),
		timeProvider:  new(MockTimeProvider),
		config:        NewDefaultBufferConfig(),
		bufferFactory: new(MockBufferFactory),
	}

	// Execute and assert the methods
	assert.True(t, manager.HasAnalysisBuffer("analysis-source"))
	assert.False(t, manager.HasAnalysisBuffer("nonexistent"))
	assert.True(t, manager.HasCaptureBuffer("capture-source"))
	assert.False(t, manager.HasCaptureBuffer("nonexistent"))
}
