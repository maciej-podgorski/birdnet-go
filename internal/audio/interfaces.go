// Package audio provides the core functionality for audio capture, processing,
// and analysis for bird sound recognition.
package audio

import (
	"time"

	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/malgo"
)

// AudioContext represents an audio context for managing audio devices.
// This interface aligns with your existing MalgoContextAdapter.
type AudioContext interface {
	// Devices returns a list of available devices.
	Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error)

	// InitDevice initializes a new audio device.
	InitDevice(config *malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error)

	// Uninit uninitializes the context.
	Uninit() error
}

// AudioDevice represents an audio device for capturing audio data.
// This interface aligns with your existing MalgoDeviceAdapter.
type AudioDevice interface {
	// Start starts audio capture.
	Start() error

	// Stop stops audio capture.
	Stop() error

	// Uninit uninitializes the audio device.
	Uninit() error
}

// AudioContextFactory creates audio contexts.
// This interface aligns with your existing factory pattern.
type AudioContextFactory interface {
	// CreateAudioContext creates a new audio context.
	CreateAudioContext(logger func(string)) (AudioContext, error)
}

// FFmpegMonitorInterface defines the interface for FFmpeg process monitoring.
// This matches your existing interface in audio_service.go.
type FFmpegMonitorInterface interface {
	Start()
	Stop()
	IsRunning() bool
}

// BufferManager defines the interface for managing audio buffers.
type BufferManager interface {
	// AllocateAnalysisBuffer initializes a ring buffer for a source.
	AllocateAnalysisBuffer(capacity int, source string) error

	// AllocateCaptureBuffer initializes a capture buffer for a source.
	AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error

	// RemoveAnalysisBuffer removes an analysis buffer.
	RemoveAnalysisBuffer(source string) error

	// RemoveCaptureBuffer removes a capture buffer.
	RemoveCaptureBuffer(source string) error

	// WriteToAnalysisBuffer writes data to an analysis buffer.
	WriteToAnalysisBuffer(stream string, data []byte) error

	// WriteToCaptureBuffer writes data to a capture buffer.
	WriteToCaptureBuffer(source string, data []byte) error

	// ReadFromAnalysisBuffer reads data from an analysis buffer.
	ReadFromAnalysisBuffer(stream string) ([]byte, error)

	// ReadSegmentFromCaptureBuffer reads a segment from a capture buffer.
	ReadSegmentFromCaptureBuffer(source string, requestedStartTime time.Time, duration int) ([]byte, error)

	// CleanupAllBuffers removes all buffers.
	CleanupAllBuffers()

	// HasAnalysisBuffer checks if a source has an analysis buffer.
	HasAnalysisBuffer(source string) bool

	// HasCaptureBuffer checks if a source has a capture buffer.
	HasCaptureBuffer(source string) bool
}

// StreamManager manages media streams.
type StreamManager interface {
	// SetCallbacks sets the callbacks for stream events.
	SetCallbacks(
		onData func(sourceID, sourceName string, data []byte),
		onLevel func(levelData AudioLevelData),
		onRestart func(),
	)

	// Start starts the stream manager.
	Start() error

	// Stop stops the stream manager.
	Stop()

	// StartStream starts capture from a stream.
	StartStream(url, transport string) error

	// StopStream stops capture from a stream.
	StopStream(url string) error

	// StopAllStreams stops all active streams.
	StopAllStreams()

	// IsStreamActive checks if a stream is active.
	IsStreamActive(url string) bool

	// ListActiveStreams returns a list of active streams.
	ListActiveStreams() []string

	// GetActiveStreams returns a list of active stream IDs.
	GetActiveStreams() []string
}

// DeviceManager manages audio capture devices.
type DeviceManager interface {
	// SetDataCallback sets the data callback function.
	SetDataCallback(callback func(sourceID string, data []byte, frameCount uint32))

	// StartCapture starts audio capture from a device.
	StartCapture(deviceID string, sampleRate, channels uint32) error

	// StopCapture stops audio capture from a device.
	StopCapture(deviceID string) error

	// ListDevices returns a list of available audio devices.
	ListDevices() ([]DeviceInfo, error)

	// GetDevice returns information about a device.
	GetDevice(deviceID string) (DeviceInfo, error)

	// Close closes the device manager and stops all devices.
	Close() error

	// GetPlatformSpecificDevices returns a list of platform-specific devices.
	GetPlatformSpecificDevices() ([]DeviceInfo, error)
}

// Processor processes audio data.
type Processor interface {
	// Process processes audio data.
	Process(data []byte, startTime time.Time, source string) error

	// ApplyFilters applies audio filters to data.
	ApplyFilters(data []byte) error

	// CalculateAudioLevel calculates audio level from data.
	CalculateAudioLevel(data []byte, source, name string) AudioLevelData

	// ConvertToFloat32 converts PCM data to float32 format for BirdNET.
	ConvertToFloat32(data []byte, bitDepth int) ([][]float32, error)
}

// ModelManager manages BirdNET model instances.
type ModelManager interface {
	// SetDefaultInstance sets the default BirdNET instance.
	SetDefaultInstance(instance *birdnet.BirdNET)

	// AssignModelToSource assigns a model ID to a source ID.
	AssignModelToSource(sourceID, modelID string)

	// RegisterModelInstance registers a BirdNET instance with a model ID.
	RegisterModelInstance(modelID string, instance *birdnet.BirdNET)

	// GetModelForSource returns the BirdNET instance for a source.
	GetModelForSource(sourceID string) *birdnet.BirdNET

	// ListModelIDs returns a list of registered model IDs.
	ListModelIDs() []string

	// GetModelInstance returns a model instance by ID.
	GetModelInstance(modelID string) *birdnet.BirdNET

	// GetDefaultModelInstance returns the default model instance.
	GetDefaultModelInstance() *birdnet.BirdNET

	// ConfigureModelForSource creates and configures a BirdNET instance.
	ConfigureModelForSource(sourceID, modelID, modelPath, labelPath string) error

	// Cleanup cleans up all model instances.
	Cleanup()
}
