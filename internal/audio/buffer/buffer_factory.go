package buffer

import (
	"time"

	"github.com/smallnest/ringbuffer"
)

// BufferFactory creates properly configured buffer instances
type BufferFactory struct {
	logger            Logger
	timeProvider      TimeProvider
	config            *BufferConfig
	ringBufferFactory func(size int) RingBufferInterface
}

// NewBufferFactory creates a buffer factory with default dependencies
func NewBufferFactory() *BufferFactory {
	return NewBufferFactoryWithDeps(
		&StandardLogger{},
		&RealTimeProvider{},
		NewDefaultBufferConfig(),
		func(size int) RingBufferInterface { return ringbuffer.New(size) },
	)
}

// NewBufferFactoryWithDeps creates a buffer factory with custom dependencies
func NewBufferFactoryWithDeps(
	logger Logger,
	timeProvider TimeProvider,
	config *BufferConfig,
	ringBufferFactory func(size int) RingBufferInterface,
) *BufferFactory {
	return &BufferFactory{
		logger:            logger,
		timeProvider:      timeProvider,
		config:            config,
		ringBufferFactory: ringBufferFactory,
	}
}

// CreateAnalysisBuffer creates a new analysis buffer
func (f *BufferFactory) CreateAnalysisBuffer(
	sampleRate, channels uint32,
	duration time.Duration,
) AnalysisBufferInterface {
	return NewAnalysisBufferWithDeps(
		sampleRate,
		channels,
		duration,
		f.ringBufferFactory,
		f.logger,
		f.timeProvider,
	)
}

// CreateCaptureBuffer creates a new capture buffer
func (f *BufferFactory) CreateCaptureBuffer(
	sampleRate, channels uint32,
	duration time.Duration,
) CaptureBufferInterface {
	return NewCaptureBufferWithDeps(
		sampleRate,
		channels,
		duration,
		f.logger,
		f.timeProvider,
	)
}

// CreateBufferManager creates a new buffer manager
func (f *BufferFactory) CreateBufferManager() BufferManagerInterface {
	return NewBufferManagerWithDeps(
		f.logger,
		f.timeProvider,
		f.config,
		f,
	)
}
