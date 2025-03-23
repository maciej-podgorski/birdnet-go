package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// FFmpegSource implements Source for FFmpeg-based streams
type FFmpegSource struct {
	config         Config
	processStart   func(context.Context, Config) (io.ReadCloser, error)
	processMu      sync.Mutex
	reader         io.ReadCloser
	active         bool
	lastData       time.Time
	watchdogCtx    context.Context
	watchdogCancel context.CancelFunc
}

// FFmpegProcessStarter is a function that starts an FFmpeg process
type FFmpegProcessStarter func(context.Context, Config) (io.ReadCloser, error)

// NewFFmpegSource creates a new FFmpeg-based stream source
func NewFFmpegSource(config *Config, processStarter FFmpegProcessStarter) *FFmpegSource {
	if config.InactiveTime == 0 {
		// Default inactive time if not specified
		config.InactiveTime = 60 * time.Second
	}

	return &FFmpegSource{
		config:       *config,
		processStart: processStarter,
		lastData:     time.Now(),
	}
}

// ID returns the source ID
func (s *FFmpegSource) ID() string {
	return s.config.ID
}

// Name returns the source name
func (s *FFmpegSource) Name() string {
	return s.config.Name
}

// IsActive returns whether the stream is active
func (s *FFmpegSource) IsActive() bool {
	s.processMu.Lock()
	defer s.processMu.Unlock()
	return s.active
}

// Start starts the stream source
func (s *FFmpegSource) Start(ctx context.Context) (io.ReadCloser, error) {
	s.processMu.Lock()
	defer s.processMu.Unlock()

	if s.active {
		return nil, errors.New("source already active")
	}

	// Start the FFmpeg process
	reader, err := s.processStart(ctx, s.config)
	if err != nil {
		return nil, fmt.Errorf("failed to start stream: %w", err)
	}

	s.reader = NewWatchedReader(reader, s.updateLastDataTime)
	s.active = true
	s.lastData = time.Now()

	// Start watchdog
	s.watchdogCtx, s.watchdogCancel = context.WithCancel(ctx)
	go s.runWatchdog()

	return s.reader, nil
}

// Stop stops the stream source
func (s *FFmpegSource) Stop() error {
	s.processMu.Lock()
	defer s.processMu.Unlock()

	if !s.active {
		return nil
	}

	// Stop watchdog
	if s.watchdogCancel != nil {
		s.watchdogCancel()
		s.watchdogCancel = nil
	}

	// Close reader
	var err error
	if s.reader != nil {
		err = s.reader.Close()
		s.reader = nil
	}

	s.active = false
	return err
}

// updateLastDataTime updates the timestamp of last received data
func (s *FFmpegSource) updateLastDataTime() {
	s.processMu.Lock()
	defer s.processMu.Unlock()
	s.lastData = time.Now()
}

// runWatchdog monitors the stream for inactivity
func (s *FFmpegSource) runWatchdog() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.watchdogCtx.Done():
			return
		case <-ticker.C:
			func() {
				s.processMu.Lock()
				defer s.processMu.Unlock()

				// Check if stream is inactive for too long
				if s.active && time.Since(s.lastData) > s.config.InactiveTime {
					fmt.Printf("Stream %s inactive for %v, triggering restart\n", s.config.ID, s.config.InactiveTime)

					// We need to release the lock before calling Stop/Start to avoid deadlock
					// Store the current state
					active := s.active
					s.processMu.Unlock()

					// Only restart if still active
					if active {
						if err := s.Stop(); err != nil {
							fmt.Printf("Error stopping stream %s: %v\n", s.config.ID, err)
							// Continue with restart attempt despite the error
						}
						ctx := context.Background()
						_, _ = s.Start(ctx) // Ignore errors, we'll retry next tick if needed
					}

					// Reacquire the lock so the deferred unlock works properly
					s.processMu.Lock()
				}
			}()
		}
	}
}

// WatchedReader wraps an io.ReadCloser and calls a notification function on each read
type WatchedReader struct {
	reader io.ReadCloser
	notify func()
}

// NewWatchedReader creates a new watched reader
func NewWatchedReader(reader io.ReadCloser, notify func()) *WatchedReader {
	return &WatchedReader{
		reader: reader,
		notify: notify,
	}
}

// Read implements io.Reader
func (w *WatchedReader) Read(p []byte) (n int, err error) {
	n, err = w.reader.Read(p)
	if n > 0 && w.notify != nil {
		w.notify()
	}
	return
}

// Close implements io.Closer
func (w *WatchedReader) Close() error {
	return w.reader.Close()
}
