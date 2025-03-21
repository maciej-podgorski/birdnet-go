package buffer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCaptureBuffer_Write_FirstWrite(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup test data
	testData := []byte{1, 2, 3, 4, 5}
	currentTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup expectations
	mockTimeProvider.On("Now").Return(currentTime)

	// Create buffer
	buffer := &CaptureBuffer{
		data:           make([]byte, 1000),
		writeIndex:     0,
		sampleRate:     48000,
		channels:       1,
		bytesPerSample: 2,
		bufferSize:     1000,
		bufferDuration: 3 * time.Second,
		initialized:    false,
		logger:         mockLogger,
		timeProvider:   mockTimeProvider,
	}

	// Execute the method
	n, err := buffer.Write(testData)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.True(t, buffer.initialized)
	assert.Equal(t, currentTime, buffer.startTime)
	assert.Equal(t, 5, buffer.writeIndex)

	// Check the data was actually written
	assert.Equal(t, testData, buffer.data[:5])

	// Verify expectations
	mockTimeProvider.AssertExpectations(t)
}

func TestCaptureBuffer_Write_WithWrapAround(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup test data
	testData := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	currentTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	startTime := time.Date(2023, 1, 1, 11, 59, 57, 0, time.UTC) // 3 seconds earlier

	// Setup expectations
	mockTimeProvider.On("Now").Return(currentTime)

	// Create buffer with writeIndex near the end to force wrap-around
	buffer := &CaptureBuffer{
		data:           make([]byte, 10),
		writeIndex:     7, // Only 3 slots left, will need to wrap around
		sampleRate:     48000,
		channels:       1,
		bytesPerSample: 2,
		bufferSize:     10,
		bufferDuration: 3 * time.Second,
		initialized:    true,
		startTime:      startTime,
		logger:         mockLogger,
		timeProvider:   mockTimeProvider,
	}

	// Execute the method
	n, err := buffer.Write(testData)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Check wrap-around
	assert.Equal(t, 5, buffer.writeIndex) // Started at 7, wrote 8 bytes, wrapped at 10, so now at index 5

	// Check that data was written correctly with wrap-around
	assert.Equal(t, byte(1), buffer.data[7])
	assert.Equal(t, byte(2), buffer.data[8])
	assert.Equal(t, byte(3), buffer.data[9])
	assert.Equal(t, byte(4), buffer.data[0])
	assert.Equal(t, byte(5), buffer.data[1])
	assert.Equal(t, byte(6), buffer.data[2])
	assert.Equal(t, byte(7), buffer.data[3])
	assert.Equal(t, byte(8), buffer.data[4])

	// Check that start time was adjusted
	assert.Equal(t, currentTime.Add(-3*time.Second), buffer.startTime)

	// Verify expectations
	mockTimeProvider.AssertExpectations(t)
}

func TestCaptureBuffer_ReadSegment_Success(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Setup times
	startTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	currentTime := time.Date(2023, 1, 1, 12, 0, 5, 0, time.UTC)  // 5 seconds after start
	requestStart := time.Date(2023, 1, 1, 12, 0, 1, 0, time.UTC) // 1 second after start

	// Sample rate: 48kHz, stereo, 16-bit samples = 192000 bytes/sec
	bytesPerSecond := 48000 * 1 * 2

	// Setup expectations for time provider
	mockTimeProvider.On("Now").Return(currentTime)

	// Create a buffer with sample data
	buffer := &CaptureBuffer{
		data:           make([]byte, 5*bytesPerSecond), // 5 seconds of data
		writeIndex:     5 * bytesPerSecond,             // Full buffer
		sampleRate:     48000,
		channels:       1,
		bytesPerSample: 2,
		bufferSize:     5 * bytesPerSecond,
		bufferDuration: 5 * time.Second,
		initialized:    true,
		startTime:      startTime,
		logger:         mockLogger,
		timeProvider:   mockTimeProvider,
	}

	// Fill buffer with test pattern
	for i := 0; i < 5*bytesPerSecond; i++ {
		buffer.data[i] = byte(i % 256)
	}

	// Execute the method
	segment, err := buffer.ReadSegment(requestStart, 2) // 2 seconds

	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, segment)
	assert.Equal(t, 2*bytesPerSecond, len(segment)) // 2 seconds of data

	// Verify the segment contains the expected data (from 1 sec to 3 sec in the buffer)
	for i := 0; i < 2*bytesPerSecond; i++ {
		expectedPos := bytesPerSecond + i // Start at 1 sec offset
		assert.Equal(t, byte(expectedPos%256), segment[i], "Byte at position %d incorrect", i)
	}

	// Verify expectations
	mockTimeProvider.AssertExpectations(t)
}

func TestCaptureBuffer_ReadSegment_NotInitialized(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Create buffer that's not initialized
	buffer := &CaptureBuffer{
		data:           make([]byte, 1000),
		writeIndex:     0,
		sampleRate:     48000,
		channels:       1,
		bytesPerSample: 2,
		bufferSize:     1000,
		bufferDuration: 3 * time.Second,
		initialized:    false, // Not initialized
		logger:         mockLogger,
		timeProvider:   mockTimeProvider,
	}

	// Execute the method
	segment, err := buffer.ReadSegment(time.Now(), 2)

	// Assert results
	assert.Error(t, err)
	assert.Nil(t, segment)
	assert.Contains(t, err.Error(), "buffer not initialized")
}

func TestCaptureBuffer_Reset(t *testing.T) {
	// Create mocks
	mockLogger := new(MockLogger)
	mockTimeProvider := new(MockTimeProvider)

	// Create buffer that's initialized with data
	buffer := &CaptureBuffer{
		data:           []byte{1, 2, 3, 4, 5},
		writeIndex:     5,
		sampleRate:     48000,
		channels:       1,
		bytesPerSample: 2,
		bufferSize:     10,
		bufferDuration: 3 * time.Second,
		initialized:    true,
		startTime:      time.Now(),
		logger:         mockLogger,
		timeProvider:   mockTimeProvider,
	}

	// Execute the method
	err := buffer.Reset()

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, 0, buffer.writeIndex)
	assert.False(t, buffer.initialized)

	// The data array should still exist (not reset)
	assert.Equal(t, []byte{1, 2, 3, 4, 5}, buffer.data)
}
