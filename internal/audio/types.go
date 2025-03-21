package audio

// DeviceInfo contains information about an audio device.
type DeviceInfo struct {
	ID       string
	Name     string
	IsInput  bool
	Channels uint32
}

// StreamInfo contains information about a media stream.
type StreamInfo struct {
	ID       string
	URL      string
	Name     string
	IsActive bool
}

// SourceType identifies the type of audio source.
type SourceType int

const (
	// SourceTypeDevice represents a physical audio device.
	SourceTypeDevice SourceType = iota

	// SourceTypeStream represents a media stream.
	SourceTypeStream
)

// SourceInfo contains information about an audio source.
type SourceInfo struct {
	ID        string
	Type      SourceType
	Name      string
	IsActive  bool
	IsDefault bool
}

// AudioLevelData holds audio level data
type AudioLevelData struct {
	Level    int    `json:"level"`    // 0-100
	Clipping bool   `json:"clipping"` // true if clipping is detected
	Source   string `json:"source"`   // Source identifier (e.g., "malgo" for device, or stream URL)
	Name     string `json:"name"`     // Human-readable name of the source
}

// FilterType identifies the type of audio filter.
type FilterType int

const (
	// FilterTypeNoise represents a noise reduction filter.
	FilterTypeNoise FilterType = iota

	// FilterTypeEqualizer represents an equalizer filter.
	FilterTypeEqualizer

	// FilterTypeHighPass represents a high-pass filter.
	FilterTypeHighPass

	// FilterTypeLowPass represents a low-pass filter.
	FilterTypeLowPass
)

// captureSource holds information about an audio capture source.
// This is adapted from your existing captureSource struct.
type CaptureSource struct {
	Name    string
	ID      string
	Pointer interface{}
}

// ServiceConfig contains configuration for the audio service.
type ServiceConfig struct {
	Debug             bool
	DefaultSampleRate uint32
	DefaultChannels   uint32
	BufferDuration    int // in milliseconds
	DefaultDevice     string
	Streams           []StreamConfig
}

// StreamConfig contains configuration for a media stream.
type StreamConfig struct {
	URL  string
	Name string
}

// FFmpegConfig contains configuration for FFmpeg.
type FFmpegConfig struct {
	URL       string
	Transport string
}

// AudioInfo returns basic information about an audio file.
type AudioInfo struct {
	SampleRate   int
	TotalSamples int
	NumChannels  int
	BitDepth     int
}

// Common error values.
var (
	ErrBufferNotFound    = Error("buffer not found")
	ErrBufferExists      = Error("buffer already exists")
	ErrInvalidParameters = Error("invalid parameters")
	ErrStreamNotFound    = Error("stream not found")
	ErrStreamExists      = Error("stream already exists")
	ErrDeviceNotFound    = Error("device not found")
	ErrDeviceExists      = Error("device already exists")
	ErrSourceNotFound    = Error("source not found")
	ErrSourceExists      = Error("source already exists")
	ErrInvalidSourceType = Error("invalid source type")
	ErrServiceNotRunning = Error("service not running")
	ErrServiceRunning    = Error("service already running")
)

// Error type for common errors.
type Error string

// Error implements the error interface.
func (e Error) Error() string {
	return string(e)
}
