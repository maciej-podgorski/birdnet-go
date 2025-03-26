package ffmpeg_test

import (
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/stretchr/testify/mock"

	"github.com/tphakala/birdnet-go/internal/audio/ffmpeg"
)

// MockCommandExecutor implements the CommandExecutor interface for testing
type MockCommandExecutor struct {
	mock.Mock
}

// Command creates a mock command
func (m *MockCommandExecutor) Command(name string, args ...string) ffmpeg.Commander {
	mockArgs := m.Called(name, args)
	return mockArgs.Get(0).(ffmpeg.Commander)
}

// MockCommander implements the Commander interface for testing
type MockCommander struct {
	mock.Mock
	stdoutPipe io.ReadCloser
	stderrPipe io.ReadCloser
	stdinPipe  io.WriteCloser
	mockProc   *MockProcess
}

// NewMockCommander creates a new MockCommander with the given pipes
func NewMockCommander(stdout, stderr io.ReadCloser, stdin io.WriteCloser) *MockCommander {
	return &MockCommander{
		stdoutPipe: stdout,
		stderrPipe: stderr,
		stdinPipe:  stdin,
		mockProc:   &MockProcess{},
	}
}

// Start simulates starting a command
func (m *MockCommander) Start() error {
	args := m.Called()
	return args.Error(0)
}

// Wait simulates waiting for a command to complete
func (m *MockCommander) Wait() error {
	args := m.Called()
	return args.Error(0)
}

// StdoutPipe returns the stdout pipe
func (m *MockCommander) StdoutPipe() (io.ReadCloser, error) {
	args := m.Called()
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return m.stdoutPipe, nil
}

// StderrPipe returns the stderr pipe
func (m *MockCommander) StderrPipe() (io.ReadCloser, error) {
	args := m.Called()
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return m.stderrPipe, nil
}

// StdinPipe returns the stdin pipe
func (m *MockCommander) StdinPipe() (io.WriteCloser, error) {
	args := m.Called()
	if args.Get(1) != nil {
		return nil, args.Error(1)
	}
	return m.stdinPipe, nil
}

// SetSysProcAttr sets the process attributes
func (m *MockCommander) SetSysProcAttr(attr *syscall.SysProcAttr) {
	m.Called(attr)
}

// Process returns the mock process
func (m *MockCommander) Process() *os.Process {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}

	// Return our mock process converted to os.Process
	// In real usage, you'd need a more sophisticated approach
	return &os.Process{Pid: m.mockProc.Pid()}
}

// MockProcess implements a mock process
type MockProcess struct {
	mock.Mock
	pid int
}

// SetPid sets the mock process ID
func (m *MockProcess) SetPid(pid int) {
	m.pid = pid
}

// Pid returns the mock process ID
func (m *MockProcess) Pid() int {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Int(0)
	}
	return m.pid
}

// Kill simulates killing the process
func (m *MockProcess) Kill() error {
	args := m.Called()
	return args.Error(0)
}

// MockReadCloser implements a mock io.ReadCloser
type MockReadCloser struct {
	mock.Mock
	reader io.Reader
}

// NewMockReadCloser creates a new MockReadCloser with the given string content
func NewMockReadCloser(content string) *MockReadCloser {
	return &MockReadCloser{
		reader: strings.NewReader(content),
	}
}

// Read reads from the underlying reader
func (m *MockReadCloser) Read(p []byte) (int, error) {
	return m.reader.Read(p)
}

// Close simulates closing the reader
func (m *MockReadCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockWriteCloser implements a mock io.WriteCloser
type MockWriteCloser struct {
	mock.Mock
	writer io.Writer
}

// NewMockWriteCloser creates a new MockWriteCloser with the given writer
func NewMockWriteCloser(writer io.Writer) *MockWriteCloser {
	return &MockWriteCloser{
		writer: writer,
	}
}

// Write writes to the underlying writer
func (m *MockWriteCloser) Write(p []byte) (int, error) {
	return m.writer.Write(p)
}

// Close simulates closing the writer
func (m *MockWriteCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}
