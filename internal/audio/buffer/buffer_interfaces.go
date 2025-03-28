package buffer

import "time"

// RingBufferInterface abstracts the ring buffer operations
type RingBufferInterface interface {
	Write(p []byte) (n int, err error)
	Read(p []byte) (n int, err error)
	Length() int
	Capacity() int
	Free() int
	Reset()
}

// BufferFactoryInterface defines common factory operations
type BufferFactoryInterface interface {
	CreateAnalysisBuffer(sampleRate, channels uint32, duration time.Duration) AnalysisBufferInterface
	CreateCaptureBuffer(sampleRate, channels uint32, duration time.Duration) CaptureBufferInterface
	CreateBufferManager() BufferManagerInterface
}

// AudioBuffer defines common operations for all buffer types
type AudioBuffer interface {
	Write(data []byte) (int, error)
	Read(p []byte) (int, error)
	Reset() error
	SampleRate() uint32
	Channels() uint32
}

// AnalysisBufferInterface extends AudioBuffer with analysis-specific methods
type AnalysisBufferInterface interface {
	AudioBuffer
	ReadyForAnalysis() bool
	Capacity() int
}

// CaptureBufferInterface extends AudioBuffer with capture-specific methods
type CaptureBufferInterface interface {
	AudioBuffer
	ReadSegment(startTime time.Time, durationSeconds int) ([]byte, error)
}

// BufferManagerInterface defines operations for managing multiple buffers
type BufferManagerInterface interface {
	AllocateAnalysisBuffer(capacity int, source string) error
	RemoveAnalysisBuffer(source string) error
	AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error
	RemoveCaptureBuffer(source string) error
	WriteToAnalysisBuffer(stream string, data []byte) error
	ReadFromAnalysisBuffer(stream string, config *BufferConfig) ([]byte, error)
	WriteToCaptureBuffer(source string, data []byte) error
	ReadSegmentFromCaptureBuffer(source string, startTime time.Time, duration int) ([]byte, error)
	CleanupAllBuffers()
	HasAnalysisBuffer(source string) bool
	HasCaptureBuffer(source string) bool
	GetAnalysisBufferCapacity(source string) (int, error)
}
