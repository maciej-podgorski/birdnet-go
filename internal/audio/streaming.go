package audio

import (
	"context"
	"time"

	"github.com/tphakala/birdnet-go/internal/audio/ffmpeg"
	"github.com/tphakala/birdnet-go/internal/audio/ffmpegstream"
	"github.com/tphakala/birdnet-go/internal/audio/stream"
)

// FFmpegStreamManager implements the StreamManager interface using ffmpegstream
type FFmpegStreamManager struct {
	manager     *ffmpegstream.FFmpegManager
	ctx         context.Context
	cancel      context.CancelFunc
	onRestartFn func()
}

// NewFFmpegStreamManager creates a new FFmpeg stream manager
func NewFFmpegStreamManager(ffmpegPath string) *FFmpegStreamManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &FFmpegStreamManager{
		manager: ffmpegstream.NewFFmpegManager(ffmpegPath),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// SetCallbacks sets the callbacks for stream events
func (m *FFmpegStreamManager) SetCallbacks(
	onData func(sourceID, sourceName string, data []byte),
	onLevel func(levelData AudioLevelData),
	onRestart func(),
) {
	m.onRestartFn = onRestart

	// Data callback adapter
	dataCallback := func(sourceID string, data []byte) {
		if onData != nil {
			source, err := m.manager.GetStream(sourceID)
			name := sourceID // Default to ID if we can't get the name
			if err == nil && source != nil {
				name = source.Name()
			}
			onData(sourceID, name, data)
		}
	}

	// Level callback adapter
	levelCallback := func(level stream.AudioLevelData) {
		if onLevel != nil {
			onLevel(AudioLevelData{
				Level:    level.Level,
				Clipping: level.Clipping,
				Source:   level.Source,
				Name:     level.Name,
			})
		}
	}

	// Set callbacks on the manager
	m.manager.SetCallbacks(dataCallback, levelCallback)
}

// Start starts the stream manager
func (m *FFmpegStreamManager) Start() error {
	return m.manager.Start(m.ctx)
}

// Stop stops the stream manager
func (m *FFmpegStreamManager) Stop() {
	m.cancel()
	// Create new context for potential future use
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.manager.Stop()
}

// StartStream starts capture from a stream
func (m *FFmpegStreamManager) StartStream(url, transport string) error {
	config := stream.Config{
		ID:        url,
		Name:      url,
		URL:       url,
		Transport: transport,
		Format: stream.AudioFormat{
			SampleRate: 48000,
			Channels:   1,
			BitDepth:   16,
		},
		InactiveTime: 60 * time.Second,
	}

	return m.manager.AddStream(config)
}

// StopStream stops capture from a stream
func (m *FFmpegStreamManager) StopStream(url string) error {
	return m.manager.RemoveStream(url)
}

// StopAllStreams stops all active streams
func (m *FFmpegStreamManager) StopAllStreams() {
	sources := m.manager.ListStreams()
	for _, source := range sources {
		m.manager.RemoveStream(source.ID())
	}
}

// IsStreamActive checks if a stream is active
func (m *FFmpegStreamManager) IsStreamActive(url string) bool {
	source, err := m.manager.GetStream(url)
	if err != nil {
		return false
	}
	return source.IsActive()
}

// ListActiveStreams returns a list of active streams
func (m *FFmpegStreamManager) ListActiveStreams() []string {
	sources := m.manager.ListStreams()
	activeStreams := make([]string, 0, len(sources))

	for _, source := range sources {
		if source.IsActive() {
			activeStreams = append(activeStreams, source.ID())
		}
	}

	return activeStreams
}

// GetActiveStreams returns a list of active streams
func (m *FFmpegStreamManager) GetActiveStreams() []string {
	return m.ListActiveStreams()
}

// FFmpegMonitor implements FFmpegMonitorInterface using ffmpeg.Monitor
type FFmpegMonitor struct {
	monitor *ffmpeg.Monitor
}

// NewFFmpegMonitor creates a new FFmpeg monitor
func NewFFmpegMonitor() *FFmpegMonitor {
	return &FFmpegMonitor{
		monitor: ffmpeg.NewMonitor(ffmpeg.MonitorOptions{
			Interval: 30 * time.Second,
		}),
	}
}

// Start starts the FFmpeg monitor
func (m *FFmpegMonitor) Start() {
	m.monitor.Start()
}

// Stop stops the FFmpeg monitor
func (m *FFmpegMonitor) Stop() {
	m.monitor.Stop()
}

// IsRunning returns whether the monitor is running
func (m *FFmpegMonitor) IsRunning() bool {
	return m.monitor.IsRunning()
}

// CreateAudioComponents creates and initializes all audio components
func CreateAudioComponents(ffmpegPath string) (StreamManager, FFmpegMonitorInterface) {
	streamManager := NewFFmpegStreamManager(ffmpegPath)
	ffmpegMonitor := NewFFmpegMonitor()

	// Start the FFmpeg monitor
	ffmpegMonitor.Start()

	return streamManager, ffmpegMonitor
}
