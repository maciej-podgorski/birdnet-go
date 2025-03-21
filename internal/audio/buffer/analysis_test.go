package buffer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAnalysisBuffer_Write_Success(t *testing.T) {
	// Create mocks
	mockRingBuffer := new(MockRingBufferInterface)
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup test data
	testData := []byte{1, 2, 3, 4, 5}

	// Setup expectations
	mockRingBuffer.On("Capacity").Return(1000)
	mockRingBuffer.On("Length").Return(0)
	mockRingBuffer.On("Write", testData).Return(len(testData), nil)

	// Create buffer with mocks
	buffer := &AnalysisBuffer{
		buffer:          mockRingBuffer,
		prevData:        nil,
		sampleRate:      48000,
		channels:        2,
		threshold:       750,
		overlapFraction: 0.25,
		warningCounter:  0,
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
	}

	// Execute the method
	n, err := buffer.Write(testData)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Verify all expectations were met
	mockRingBuffer.AssertExpectations(t)
}

func TestAnalysisBuffer_Write_Error(t *testing.T) {
	// Create mocks
	mockRingBuffer := new(MockRingBufferInterface)
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup test data
	testData := []byte{1, 2, 3, 4, 5}
	expectedError := errors.New("buffer is full")

	// Setup expectations
	mockRingBuffer.On("Capacity").Return(1000)
	mockRingBuffer.On("Length").Return(900)
	mockRingBuffer.On("Write", testData).Return(0, expectedError)
	mockRingBuffer.On("Free").Return(0)
	mockRingBuffer.On("Length").Return(1000)

	// Setup logger expectations for error messages
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Setup time provider expectations for retry delays
	mockTimeProvider.On("Sleep", 10*time.Millisecond).Return()

	// Create buffer with mocks
	buffer := &AnalysisBuffer{
		buffer:          mockRingBuffer,
		prevData:        nil,
		sampleRate:      48000,
		channels:        2,
		threshold:       750,
		overlapFraction: 0.25,
		warningCounter:  0,
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
	}

	// Execute the method
	n, err := buffer.Write(testData)

	// Assert results
	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "failed to write to buffer")

	// Verify expectations
	mockRingBuffer.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockTimeProvider.AssertExpectations(t)
}

func TestAnalysisBuffer_Read_WithEnoughData(t *testing.T) {
	// Create mocks
	mockRingBuffer := new(MockRingBufferInterface)
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup test data
	readBuffer := make([]byte, 100)
	readData := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Create buffer with existing previous data
	buffer := &AnalysisBuffer{
		buffer:          mockRingBuffer,
		prevData:        []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Previous data to use
		sampleRate:      48000,
		channels:        2,
		threshold:       750,
		overlapFraction: 0.25,
		warningCounter:  0,
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
	}

	// Setup expectations
	mockRingBuffer.On("Length").Return(200) // Plenty of data
	mockRingBuffer.On("Read", mock.Anything).Run(func(args mock.Arguments) {
		// Simulate reading data by copying test data to the provided buffer
		buf := args.Get(0).([]byte)
		copy(buf, readData)
	}).Return(len(readData), nil)

	// Execute the method
	n, err := buffer.Read(readBuffer)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, len(readBuffer), n)

	// Verify expectations
	mockRingBuffer.AssertExpectations(t)
}

func TestAnalysisBuffer_ReadyForAnalysis(t *testing.T) {
	// Create mocks
	mockRingBuffer := new(MockRingBufferInterface)
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Create buffer
	buffer := &AnalysisBuffer{
		buffer:          mockRingBuffer,
		prevData:        nil,
		sampleRate:      48000,
		channels:        2,
		threshold:       750,
		overlapFraction: 0.25,
		warningCounter:  0,
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
	}

	// Test 1: Not ready
	mockRingBuffer.On("Length").Return(500).Once()
	ready := buffer.ReadyForAnalysis()
	assert.False(t, ready)

	// Test 2: Ready
	mockRingBuffer.On("Length").Return(800).Once()
	ready = buffer.ReadyForAnalysis()
	assert.True(t, ready)

	// Verify expectations
	mockRingBuffer.AssertExpectations(t)
}

func TestAnalysisBuffer_Reset(t *testing.T) {
	// Create mocks
	mockRingBuffer := new(MockRingBufferInterface)
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Create buffer with previous data and warning count
	buffer := &AnalysisBuffer{
		buffer:          mockRingBuffer,
		prevData:        []byte{1, 2, 3, 4, 5},
		sampleRate:      48000,
		channels:        2,
		threshold:       750,
		overlapFraction: 0.25,
		warningCounter:  10,
		logger:          mockLogger,
		timeProvider:    mockTimeProvider,
	}

	// Setup expectations
	mockRingBuffer.On("Reset").Return()

	// Execute the method
	err := buffer.Reset()

	// Assert results
	assert.NoError(t, err)
	assert.Nil(t, buffer.prevData)
	assert.Equal(t, 0, buffer.warningCounter)

	// Verify expectations
	mockRingBuffer.AssertExpectations(t)
}
