package ffmpeg_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tphakala/birdnet-go/internal/audio/ffmpeg"
)

func TestProcessLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("successful start and stop", func(t *testing.T) {
		t.Parallel()

		// Create pipes for stdout, stderr, stdin
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		_, stdinW := io.Pipe()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)
		mockProc := &MockProcess{}
		mockCmd.mockProc = mockProc

		// Set up expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(nil)
		mockCmd.On("Process").Return(&os.Process{Pid: 12345})
		mockCmd.On("Wait").Return(nil)
		mockProc.On("Pid").Return(12345)
		//mockProc.On("Kill").Return(nil)

		// Create options with mock executor
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "input.mp3", "output.wav"},
			Executor:   mockExecutor,
		}

		// Start the process
		ctx := context.Background()
		proc, err := ffmpeg.Start(ctx, &opts)
		require.NoError(t, err)
		require.NotNil(t, proc)

		// Simulate FFmpeg writing to stdout
		go func() {
			_, _ = stdoutW.Write([]byte("ffmpeg output data"))
			stdoutW.Close()
		}()

		// Simulate FFmpeg writing to stderr
		go func() {
			_, _ = stderrW.Write([]byte("ffmpeg log messages"))
			stderrW.Close()
		}()

		// Wait a moment for data to be processed
		time.Sleep(50 * time.Millisecond)

		// Check if stderr was captured
		stderrOutput := proc.StderrOutput()
		assert.Contains(t, stderrOutput, "ffmpeg log messages")

		// Test stopping the process
		err = proc.Stop()
		assert.NoError(t, err)

		// Verify all expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("start process with input data", func(t *testing.T) {
		t.Parallel()

		// Create a buffer to capture stdin data
		stdinBuffer := &bytes.Buffer{}

		// Create pipes for stdout, stderr, stdin
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		stdinR, stdinW := io.Pipe()

		// Connect stdinR to our buffer to capture input
		go func() {
			_, _ = io.Copy(stdinBuffer, stdinR)
		}()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)
		mockProc := &MockProcess{}
		mockCmd.mockProc = mockProc

		// Set up expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(nil)
		mockCmd.On("Process").Return(&os.Process{Pid: 12345})
		mockCmd.On("Wait").Return(nil)
		mockProc.On("Pid").Return(12345)
		//mockProc.On("Kill").Return(nil)

		// Create input data
		inputData := []byte("test pcm audio data")

		// Create options with mock executor and input data
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "pipe:0", "output.wav"},
			InputData:  inputData,
			Executor:   mockExecutor,
		}

		// Start the process
		ctx := context.Background()
		proc, err := ffmpeg.Start(ctx, &opts)
		require.NoError(t, err)
		require.NotNil(t, proc)

		// Close pipes to simulate completion
		stdoutW.Close()
		stderrW.Close()

		// Wait a moment for data to be processed
		time.Sleep(50 * time.Millisecond)

		// Stop the process
		err = proc.Stop()
		assert.NoError(t, err)

		// Verify input data was written to stdin
		assert.Equal(t, inputData, stdinBuffer.Bytes())

		// Verify all expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("process start error", func(t *testing.T) {
		t.Parallel()

		// Create pipes for stdout, stderr, stdin
		stdoutR, _ := io.Pipe()
		stderrR, _ := io.Pipe()
		_, stdinW := io.Pipe()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)

		// Set up expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(errors.New("failed to start process"))

		// Create options with mock executor
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "input.mp3", "output.wav"},
			Executor:   mockExecutor,
		}

		// Start the process - should fail
		ctx := context.Background()
		proc, err := ffmpeg.Start(ctx, &opts)
		assert.Error(t, err)
		assert.Nil(t, proc)
		assert.Contains(t, err.Error(), "failed to start FFmpeg")

		// Verify all expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
	})

	t.Run("process stop with wait error", func(t *testing.T) {
		t.Parallel()

		// Create pipes for stdout, stderr, stdin
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		_, stdinW := io.Pipe()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)
		mockProc := &MockProcess{}
		mockCmd.mockProc = mockProc

		// Set up expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(nil)
		mockCmd.On("Process").Return(&os.Process{Pid: 12345})
		mockCmd.On("Wait").Return(errors.New("process exited with code 1"))
		mockProc.On("Pid").Return(12345)

		// Write to stderr to test error message inclusion
		go func() {
			stderrW.Write([]byte("ffmpeg error: Invalid argument"))
			stderrW.Close()
		}()
		stdoutW.Close()

		// Create options with mock executor
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "input.mp3", "output.wav"},
			Executor:   mockExecutor,
		}

		// Start the process
		ctx := context.Background()
		proc, err := ffmpeg.Start(ctx, &opts)
		require.NoError(t, err)
		require.NotNil(t, proc)

		// Wait a moment for stderr to be processed
		time.Sleep(50 * time.Millisecond)

		// Stop the process - should return error
		err = proc.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process exited with code 1")
		assert.Contains(t, err.Error(), "ffmpeg error: Invalid argument")

		// Verify ProcessError unwrapping works
		var processErr *ffmpeg.ProcessError
		assert.True(t, errors.As(err, &processErr))
		assert.NotEmpty(t, processErr.Stderr)

		// Verify all expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})

	t.Run("process stop with timeout", func(t *testing.T) {
		// This test verifies that a process that doesn't exit gets forcibly killed after timeout
		t.Parallel()

		// Create pipes
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		_, stdinW := io.Pipe()

		// Close pipes immediately to avoid leaks
		stdoutW.Close()
		stderrW.Close()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)

		// Create a real os.Process with a valid PID for the platform-specific code
		mockProc := &os.Process{Pid: 12345}

		// Set up key expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(nil)
		mockCmd.On("Process").Return(mockProc)

		// Make Wait() block long enough to trigger a timeout
		mockCmd.On("Wait").Run(func(args mock.Arguments) {
			t.Log("Wait() called, sleeping to force timeout")
			time.Sleep(7 * time.Second) // Longer than the 5s timeout in Stop()
		}).Return(nil)

		// Create and start the process
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "input.mp3", "output.wav"},
			Executor:   mockExecutor,
		}

		ctx := context.Background()
		proc, err := ffmpeg.Start(ctx, &opts)
		require.NoError(t, err)
		require.NotNil(t, proc)

		// Record the start time and call Stop
		startTime := time.Now()
		t.Log("Calling Stop(), expecting timeout after ~5 seconds")

		// Call Stop - this should timeout
		err = proc.Stop()

		elapsed := time.Since(startTime)
		t.Logf("Stop() completed after %v", elapsed)

		// Verify the timeout occurred
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process killed after timeout")
		assert.True(t, elapsed >= 4*time.Second,
			"Stop should take approximately 5 seconds for timeout (took %v)", elapsed)

		// Verify basic expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
	})
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("cancel context stops process", func(t *testing.T) {
		t.Parallel()

		// Create pipes for stdout, stderr, stdin
		stdoutR, stdoutW := io.Pipe()
		stderrR, stderrW := io.Pipe()
		_, stdinW := io.Pipe()

		// Create mocks
		mockExecutor := &MockCommandExecutor{}
		mockCmd := NewMockCommander(stdoutR, stderrR, stdinW)
		mockProc := &MockProcess{}
		mockCmd.mockProc = mockProc

		// Set up expectations
		mockExecutor.On("Command", "/path/to/ffmpeg", mock.Anything).Return(mockCmd)
		mockCmd.On("StdoutPipe").Return(stdoutR, nil)
		mockCmd.On("StderrPipe").Return(stderrR, nil)
		mockCmd.On("StdinPipe").Return(stdinW, nil)
		mockCmd.On("SetSysProcAttr", mock.Anything).Return()
		mockCmd.On("Start").Return(nil)
		mockCmd.On("Process").Return(&os.Process{Pid: 12345})
		mockCmd.On("Wait").Return(context.Canceled)
		mockProc.On("Pid").Return(12345)

		// Close pipes
		stdoutW.Close()
		stderrW.Close()

		// Create cancelable context
		ctx, cancel := context.WithCancel(context.Background())

		// Create options with mock executor
		opts := ffmpeg.ProcessOptions{
			FFmpegPath: "/path/to/ffmpeg",
			Args:       []string{"-i", "input.mp3", "output.wav"},
			Executor:   mockExecutor,
		}

		// Start the process
		proc, err := ffmpeg.Start(ctx, &opts)
		require.NoError(t, err)
		require.NotNil(t, proc)

		// Cancel the context
		cancel()

		// Wait a moment for cancellation to propagate
		time.Sleep(50 * time.Millisecond)

		// Stop the process - UPDATED: We now expect context.Canceled error
		// This is the expected behavior since our mock cmd.Wait() returns context.Canceled
		err = proc.Stop()
		assert.Equal(t, context.Canceled, err, "Stop should return the context.Canceled error from Wait")

		// Verify all expectations were met
		mockExecutor.AssertExpectations(t)
		mockCmd.AssertExpectations(t)
		mockProc.AssertExpectations(t)
	})
}
