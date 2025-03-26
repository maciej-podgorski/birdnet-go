package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// Process represents an FFmpeg process
type Process struct {
	cmd             Commander
	stdout          io.ReadCloser
	stderr          io.ReadCloser
	stdin           io.WriteCloser
	stderrBuf       *BoundedBuffer
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	restartInfo     *RestartInfo
	stderrCollector sync.WaitGroup
}

// RestartInfo tracks FFmpeg restart information
type RestartInfo struct {
	count    int
	lastTime time.Time
	mu       sync.Mutex
}

// ProcessOptions contains options for starting an FFmpeg process
type ProcessOptions struct {
	FFmpegPath string
	Args       []string
	InputData  []byte          // Optional: data to write to stdin
	Executor   CommandExecutor // Optional: custom command executor
}

// ProcessError is an error with FFmpeg stderr output
type ProcessError struct {
	Err    error
	Stderr string
}

// Error implements the error interface
func (e *ProcessError) Error() string {
	return fmt.Sprintf("%v\nStderr: %s", e.Err, e.Stderr)
}

// Unwrap returns the underlying error
func (e *ProcessError) Unwrap() error {
	return e.Err
}

// BoundedBuffer is a thread-safe bounded buffer
type BoundedBuffer struct {
	data []byte
	size int
	mu   sync.Mutex
}

// NewBoundedBuffer creates a new bounded buffer with the specified size
func NewBoundedBuffer(size int) *BoundedBuffer {
	return &BoundedBuffer{
		data: make([]byte, 0, size),
		size: size,
	}
}

// Write implements io.Writer
func (b *BoundedBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(p) >= b.size {
		// If data is larger than buffer, just keep the end
		b.data = append(b.data[:0], p[len(p)-b.size:]...)
		return len(p), nil
	}

	// If buffer would overflow, make room
	if len(b.data)+len(p) > b.size {
		b.data = b.data[len(b.data)+len(p)-b.size:]
	}

	b.data = append(b.data, p...)
	return len(p), nil
}

// String returns the buffer contents as a string
func (b *BoundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

// Reset resets the buffer
func (b *BoundedBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = b.data[:0]
}

// Start starts a new FFmpeg process
func Start(ctx context.Context, opts *ProcessOptions) (*Process, error) {
	if opts.FFmpegPath == "" {
		return nil, errors.New("FFmpeg path not specified")
	}

	// Use provided executor or default
	executor := opts.Executor
	if executor == nil {
		executor = DefaultExecutor
	}

	// Create context with cancellation
	pctx, cancel := context.WithCancel(ctx)

	// Create command
	cmd := executor.Command(opts.FFmpegPath, opts.Args...)

	// Set up process group based on platform
	setupProcessGroup(cmd)

	// Create stderr buffer
	stderrBuf := NewBoundedBuffer(4096)

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Create process object
	proc := &Process{
		cmd:         cmd,
		stdout:      stdout,
		stderr:      stderr,
		stdin:       stdin,
		stderrBuf:   stderrBuf,
		ctx:         pctx,
		cancel:      cancel,
		restartInfo: &RestartInfo{lastTime: time.Now()},
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Start stderr collection in background
	proc.stderrCollector.Add(1)
	go func() {
		defer proc.stderrCollector.Done()
		proc.collectStderr()
	}()

	// Write input data if provided
	if len(opts.InputData) > 0 {
		go func() {
			defer stdin.Close()
			_, _ = stdin.Write(opts.InputData)
		}()
	}

	return proc, nil
}

// collectStderr reads stderr output and stores it in the buffer
func (p *Process) collectStderr() {
	scanner := bufio.NewScanner(p.stderr)
	for scanner.Scan() {
		line := scanner.Bytes()
		_, err := p.stderrBuf.Write(append(line, '\n'))
		if err != nil {
			// Just log the error and continue; we don't want to crash the stderr collector
			log.Printf("Error writing to stderr buffer: %v", err)
		}
	}
}

// StdoutReader returns a reader for stdout
func (p *Process) StdoutReader() io.Reader {
	return p.stdout
}

// StderrOutput returns captured stderr output
func (p *Process) StderrOutput() string {
	return p.stderrBuf.String()
}

// Stop stops the FFmpeg process
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	// Close pipes to avoid leaks
	if p.stdin != nil {
		p.stdin.Close()
		p.stdin = nil
	}

	// Wait with timeout for stderr collector to finish
	stderrDone := make(chan struct{})
	go func() {
		p.stderrCollector.Wait()
		close(stderrDone)
	}()

	select {
	case <-stderrDone:
		// Collector finished
	case <-time.After(2 * time.Second):
		// Timeout
	}

	if p.stdout != nil {
		p.stdout.Close()
		p.stdout = nil
	}

	if p.stderr != nil {
		p.stderr.Close()
		p.stderr = nil
	}

	// Wait with timeout for process to exit
	done := make(chan error, 1)
	go func() {
		if p.cmd != nil {
			proc := p.cmd.Process()
			if proc != nil {
				done <- p.cmd.Wait()
			} else {
				done <- nil
			}
		} else {
			done <- nil
		}
	}()

	var err error

	select {
	case err = <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill if it doesn't exit gracefully
		_ = killProcessGroup(p.cmd)
		err = errors.New("process killed after timeout")
	}

	if err != nil && p.stderrBuf.String() != "" {
		return &ProcessError{
			Err:    err,
			Stderr: p.stderrBuf.String(),
		}
	}

	return err
}

// updateRestartInfo updates restart tracking information
func (p *Process) updateRestartInfo() {
	p.restartInfo.mu.Lock()
	defer p.restartInfo.mu.Unlock()

	now := time.Now()
	if now.Sub(p.restartInfo.lastTime) > time.Minute {
		// Reset count if it's been more than a minute
		p.restartInfo.count = 0
	}

	p.restartInfo.count++
	p.restartInfo.lastTime = now
}

// getRestartDelay returns the recommended delay before restart based on history
func (p *Process) getRestartDelay() time.Duration {
	p.restartInfo.mu.Lock()
	defer p.restartInfo.mu.Unlock()

	// Exponential backoff with cap
	delay := time.Duration(p.restartInfo.count) * 5 * time.Second
	maxDelay := 2 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
