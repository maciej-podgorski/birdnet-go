package ffmpegstream

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/tphakala/birdnet-go/internal/audio/ffmpeg"
	"github.com/tphakala/birdnet-go/internal/audio/stream"
)

// FFmpegManager provides integration between stream management and FFmpeg process management
type FFmpegManager struct {
	ffmpegPath string
	monitor    *ffmpeg.Monitor
	streamMgr  stream.Manager
	mu         sync.Mutex
}

// NewFFmpegManager creates a new FFmpeg manager
func NewFFmpegManager(ffmpegPath string) *FFmpegManager {
	// Create the FFmpeg monitor
	monitor := ffmpeg.NewMonitor(ffmpeg.MonitorOptions{
		Interval: 30 * time.Second,
	})

	// Create the stream manager
	streamMgr := stream.NewManager()

	return &FFmpegManager{
		ffmpegPath: ffmpegPath,
		monitor:    monitor,
		streamMgr:  streamMgr,
	}
}

// Start starts the FFmpeg manager
func (m *FFmpegManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Start the monitor
	m.monitor.Start()

	// Start the stream manager
	return m.streamMgr.Start(ctx)
}

// Stop stops the FFmpeg manager
func (m *FFmpegManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop the monitor
	m.monitor.Stop()

	// Stop the stream manager
	return m.streamMgr.Stop()
}

// SetCallbacks sets callbacks for stream events
func (m *FFmpegManager) SetCallbacks(onData stream.DataCallback, onLevel stream.LevelCallback) {
	m.streamMgr.SetCallbacks(onData, onLevel)
}

// AddStream adds a stream to the manager
func (m *FFmpegManager) AddStream(config stream.Config) error {
	// Create the FFmpeg process starter function
	processStarter := func(ctx context.Context, cfg stream.Config) (io.ReadCloser, error) {
		// Create stream source configuration
		source := ffmpeg.StreamSource{
			URL: cfg.URL,
		}

		// Determine stream protocol
		switch {
		case config.Transport == "tcp" || config.Transport == "udp":
			source.Protocol = ffmpeg.StreamProtocol(config.Transport)
		case config.Transport == "hls":
			source.Protocol = ffmpeg.ProtocolHLS
		case config.Transport == "http":
			source.Protocol = ffmpeg.ProtocolHTTP
		default:
			// Default to TCP for RTSP
			source.Protocol = ffmpeg.ProtocolTCP
		}

		// Create FFmpeg stream format
		format := ffmpeg.StreamFormat{
			SampleRate: cfg.Format.SampleRate,
			Channels:   cfg.Format.Channels,
			BitDepth:   cfg.Format.BitDepth,
		}

		// Build FFmpeg command
		builder := ffmpeg.NewCommandBuilder(m.ffmpegPath)
		builder.WithInputURL(source)
		builder.WithFormat(format)
		builder.WithOutputPipe()
		builder.DisableVideo()

		// Start FFmpeg process
		process, err := ffmpeg.Start(ctx, ffmpeg.ProcessOptions{
			FFmpegPath: m.ffmpegPath,
			Args:       builder.Build(),
		})
		if err != nil {
			return nil, err
		}

		// Register the process with the monitor
		m.monitor.RegisterProcess(cfg.URL, process)

		// Return stdout reader
		return &processReadCloser{
			reader:  process.StdoutReader(),
			process: process,
			url:     cfg.URL,
			onClose: m.onProcessClosed,
		}, nil
	}

	// Create a stream source
	source := stream.NewFFmpegSource(config, processStarter)

	// Add the source to the stream manager
	return m.streamMgr.AddSource(source)
}

// RemoveStream removes a stream from the manager
func (m *FFmpegManager) RemoveStream(sourceID string) error {
	return m.streamMgr.RemoveSource(sourceID)
}

// GetStream returns a stream by ID
func (m *FFmpegManager) GetStream(sourceID string) (stream.Source, error) {
	return m.streamMgr.GetSource(sourceID)
}

// ListStreams returns all streams
func (m *FFmpegManager) ListStreams() []stream.Source {
	return m.streamMgr.ListSources()
}

// onProcessClosed is called when a process is closed
func (m *FFmpegManager) onProcessClosed(url string) {
	m.monitor.UnregisterProcess(url)
}

// processReadCloser wraps an FFmpeg process stdout reader with a proper Close method
type processReadCloser struct {
	reader  io.Reader
	process *ffmpeg.Process
	url     string
	onClose func(string)
}

func (p *processReadCloser) Read(buf []byte) (int, error) {
	return p.reader.Read(buf)
}

func (p *processReadCloser) Close() error {
	if p.process != nil {
		err := p.process.Stop()
		if p.onClose != nil {
			p.onClose(p.url)
		}
		return err
	}
	return nil
}

// ExportAudio exports PCM data to an audio file
func (m *FFmpegManager) ExportAudio(ctx context.Context, data []byte, outputPath string, format ffmpeg.ExportFormat, streamFormat stream.AudioFormat) error {
	// Convert stream format to FFmpeg format
	ffmpegFormat := ffmpeg.StreamFormat{
		SampleRate: streamFormat.SampleRate,
		Channels:   streamFormat.Channels,
		BitDepth:   streamFormat.BitDepth,
	}

	// Create export options
	opts := ffmpeg.ExportOptions{
		FFmpegPath:   m.ffmpegPath,
		Format:       format,
		StreamFormat: ffmpegFormat,
		OutputPath:   outputPath,
		Timeout:      5 * time.Minute,
	}

	// Export the audio
	return ffmpeg.Export(ctx, data, opts)
}
