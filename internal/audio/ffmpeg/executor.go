package ffmpeg

import (
	"io"
	"os"
	"os/exec"
	"syscall"
)

// CommandExecutor abstracts the creation of commands
type CommandExecutor interface {
	// Command creates a new command instance
	Command(name string, args ...string) Commander
}

// Commander abstracts the exec.Cmd functionality
type Commander interface {
	// Start starts the command but doesn't wait for it to complete
	Start() error

	// Wait waits for the command to exit and returns any error
	Wait() error

	// StdoutPipe returns a pipe connected to stdout
	StdoutPipe() (io.ReadCloser, error)

	// StderrPipe returns a pipe connected to stderr
	StderrPipe() (io.ReadCloser, error)

	// StdinPipe returns a pipe connected to stdin
	StdinPipe() (io.WriteCloser, error)

	// SetSysProcAttr sets the process attributes
	SetSysProcAttr(attr *syscall.SysProcAttr)

	// Process returns the underlying process info if available
	Process() *os.Process
}

// DefaultCommandExecutor uses the real exec.Command
type DefaultCommandExecutor struct{}

// Command creates a real exec.Cmd
func (e *DefaultCommandExecutor) Command(name string, args ...string) Commander {
	return &DefaultCommander{
		cmd: exec.Command(name, args...),
	}
}

// DefaultCommander wraps a real exec.Cmd
type DefaultCommander struct {
	cmd *exec.Cmd
}

// Start starts the command
func (c *DefaultCommander) Start() error {
	return c.cmd.Start()
}

// Wait waits for the command to complete
func (c *DefaultCommander) Wait() error {
	return c.cmd.Wait()
}

// StdoutPipe returns a pipe connected to stdout
func (c *DefaultCommander) StdoutPipe() (io.ReadCloser, error) {
	return c.cmd.StdoutPipe()
}

// StderrPipe returns a pipe connected to stderr
func (c *DefaultCommander) StderrPipe() (io.ReadCloser, error) {
	return c.cmd.StderrPipe()
}

// StdinPipe returns a pipe connected to stdin
func (c *DefaultCommander) StdinPipe() (io.WriteCloser, error) {
	return c.cmd.StdinPipe()
}

// SetSysProcAttr sets the process attributes
func (c *DefaultCommander) SetSysProcAttr(attr *syscall.SysProcAttr) {
	c.cmd.SysProcAttr = attr
}

// Process returns the underlying process
func (c *DefaultCommander) Process() *os.Process {
	return c.cmd.Process
}

// DefaultExecutor is the standard command executor
var DefaultExecutor CommandExecutor = &DefaultCommandExecutor{}
