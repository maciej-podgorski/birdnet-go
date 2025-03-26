# FFmpeg Audio Stream Processing

## Overview

This package provides a robust, testable implementation for handling audio streams using FFmpeg. The code is organized into two main packages:

1. `stream`: Generic stream handling interfaces and implementations
2. `ffmpeg`: FFmpeg-specific process management and command building

The design focuses on:
- Interface-based dependency injection for testability
- Robust error handling and recovery
- Clean separation of concerns
- Support for multiple stream formats (RTSP, HLS, HTTP, etc.)
- Proper resource management and process lifecycle handling
- Cross-platform support for Linux, macOS, and Windows

## Package Structure

```
internal/
  audio/
    stream/          # Generic stream handling
      interfaces.go  # Stream interfaces
      ffmpeg_source.go  # FFmpeg-specific source
      stream_manager.go # Stream management
    ffmpeg/          # FFmpeg process management
      process.go     # Process management
      command.go     # Command building
      export.go      # Audio export 
      monitor.go     # Process monitoring
      platform_unix.go   # Unix-specific platform code
      platform_windows.go # Windows-specific platform code
    ffmpegstream/    # Integration layer
      integration.go # Integration between stream and FFmpeg
```

## Key Components

### `stream` Package

#### Interfaces

- `Source`: Represents an audio stream source with methods to start, stop, and check status
- `Manager`: Manages multiple stream sources with callbacks for data and audio levels
- `DataCallback`: Function type for receiving audio data from streams
- `LevelCallback`: Function type for receiving audio level information

#### Implementations

- `DefaultManager`: Manages multiple stream sources with proper thread safety
- `FFmpegSource`: An implementation of `Source` using FFmpeg processes with watchdog capabilities

### `ffmpeg` Package

#### Core Components

- `Process`: Manages FFmpeg process lifecycle with proper cleanup and error handling
- `CommandBuilder`: Fluent interface for building FFmpeg commands with various options
- `Monitor`: Tracks and cleans up orphaned FFmpeg processes
- `Export`: Exports PCM data to various audio formats
- Platform-specific implementations for process handling

#### Key Types

- `StreamFormat`: Audio format parameters (sample rate, channels, bit depth)
- `StreamSource`: Input stream source information
- `StreamProtocol`: Stream protocol type (TCP, UDP, HTTP, HLS)
- `ExportFormat`: Audio export format options
- `ProcessTracker`: Interface for tracking FFmpeg processes
- `SystemProcessFinder`: Interface for finding system processes

### `ffmpegstream` Package

While this package focuses on the core FFmpeg functionality, it is designed to be integrated with the `ffmpegstream` package, which provides:

- `FFmpegManager`: Integration layer between stream management and FFmpeg process handling
- Seamless stream configuration to FFmpeg command mapping
- Simplified API for adding, removing, and managing streams
- Stream export functionality
- Automatic protocol detection and configuration

See the [FFmpegStream Package Documentation](../ffmpegstream/README.md) for details on using the integration layer.

## Usage Examples

### Creating a Stream Manager with FFmpeg Sources

```go
// Create FFmpeg manager
manager := ffmpegstream.NewFFmpegManager("/usr/bin/ffmpeg")

// Set up callbacks for stream data
manager.SetCallbacks(
    // Data callback
    func(sourceID string, data []byte) {
        fmt.Printf("Received %d bytes from %s\n", len(data), sourceID)
    },
    // Audio level callback
    func(level stream.AudioLevelData) {
        fmt.Printf("Audio level: %d%%\n", level.Level)
    },
)

// Add a stream source
streamConfig := stream.Config{
    ID: "camera1",
    Name: "Front Camera",
    URL: "rtsp://example.com/stream1",
    Transport: "tcp",
    Format: stream.AudioFormat{
        SampleRate: 48000,
        Channels: 1,
        BitDepth: 16,
    },
    InactiveTime: 60 * time.Second,
}

// Add the stream
err := manager.AddStream(streamConfig)
if err != nil {
    log.Fatalf("Failed to add stream: %v", err)
}

// Start the manager
ctx := context.Background()
err = manager.Start(ctx)
if err != nil {
    log.Fatalf("Failed to start manager: %v", err)
}

// ... Later ...
manager.Stop()
```

### Using the FFmpeg Command Builder

```go
// Create command builder
builder := ffmpeg.NewCommandBuilder("/usr/bin/ffmpeg")

// Set input URL
builder.WithInputURL(ffmpeg.StreamSource{
    URL: "rtsp://example.com/stream1",
    Protocol: ffmpeg.ProtocolTCP,
})

// Set audio format
builder.WithFormat(ffmpeg.StreamFormat{
    SampleRate: 48000,
    Channels: 1,
    BitDepth: 16,
})

// Configure output
builder.WithOutputPipe()
builder.DisableVideo()

// Get command string for debugging
fmt.Println(builder.BuildCommand())

// Get command arguments for process execution
args := builder.Build()
```

### Exporting Audio to a File

```go
// Create PCM data buffer
pcmData := []byte{...} // Your PCM audio data

// Create export options
exportOpts := ffmpeg.ExportOptions{
    FFmpegPath: "/usr/bin/ffmpeg",
    Format: ffmpeg.ExportFormat{
        Type: "mp3",
        Bitrate: "192k",
    },
    StreamFormat: ffmpeg.StreamFormat{
        SampleRate: 44100,
        Channels: 2,
        BitDepth: 16,
    },
    OutputPath: "output.mp3",
    Timeout: 60 * time.Second,
}

// Create context
ctx := context.Background()

// Export audio to file
err := ffmpeg.Export(ctx, pcmData, exportOpts)
if err != nil {
    fmt.Printf("Error exporting audio: %v\n", err)
}
```

### Using the Process Monitor

```go
// Create a process monitor
monitor := ffmpeg.NewMonitor(ffmpeg.MonitorOptions{
    Interval: 30 * time.Second,
})

// Start the monitor
monitor.Start()

// Register a process with the monitor
process, _ := ffmpeg.Start(ctx, ffmpeg.ProcessOptions{
    FFmpegPath: "/usr/bin/ffmpeg",
    Args: []string{...},
})
monitor.RegisterProcess("rtsp://example.com/stream", process)

// Later, unregister the process
monitor.UnregisterProcess("rtsp://example.com/stream")

// Stop the monitor when done
monitor.Stop()
```

## Testability

The package is designed for easy testing:

1. All dependencies are injected through interfaces
2. Core functions accept context for cancellation
3. Mocks are easy to create for each component
4. Callbacks provide easy ways to verify behavior

### Example Test

```go
func TestStreamManager(t *testing.T) {
    // Create a mock source
    source := &mockSource{id: "test", name: "Test Source"}
    
    // Create manager
    manager := stream.NewManager()
    
    // Add source
    err := manager.AddSource(source)
    if err != nil {
        t.Fatalf("Failed to add source: %v", err)
    }
    
    // Start manager
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    err = manager.Start(ctx)
    if err != nil {
        t.Fatalf("Failed to start manager: %v", err)
    }
    
    // Verify source was started
    if !source.started {
        t.Fatal("Source was not started")
    }
    
    // Stop manager
    manager.Stop()
    
    // Verify source was stopped
    if !source.stopped {
        t.Fatal("Source was not stopped")
    }
}
```

## Error Handling

The package uses structured error handling throughout:

1. All errors are wrapped with context using `fmt.Errorf` and `%w`
2. Process errors include stderr output for better diagnostics
3. Exponential backoff is used for restarts
4. Isolated error handling prevents cascading failures

## Cross-Platform Support

The package supports Linux, macOS, and Windows:

1. Platform-specific process group handling via separate implementation files
2. Different process listing and termination methods:
   - Unix: pgrep/pkill for process management
   - Windows: tasklist/taskkill for process management
3. Context-based cancellation for all platforms

## Robust Process Management

FFmpeg processes are properly managed:

1. Process groups are used to ensure child processes are also terminated
2. Watchdog detects inactive streams and triggers restarts
3. Process monitor finds and cleans up orphaned processes
4. Context cancellation is propagated to all processes
5. Stderr output is captured for better error diagnostics

## Configuration

The package is highly configurable:

1. Stream configuration includes format, transport, and timing parameters
2. Process monitoring can be customized with different intervals
3. Export supports various formats with appropriate codecs and containers
4. Command building provides flexibility for different streaming scenarios
5. Inactivity timeout for streams is configurable

## API Reference

### Types

#### Process Management

**`Process`** (`process.go`): Represents an FFmpeg process
- Fields:
  - `cmd`: Commander interface for executing commands
  - `stdout`, `stderr`, `stdin`: I/O pipes for the process
  - `stderrBuf`: Buffer to collect stderr output
  - Other internal fields for concurrency management

**`ProcessOptions`** (`process.go`): Options for starting an FFmpeg process
- Fields:
  - `FFmpegPath`: Path to the FFmpeg executable
  - `Args`: Command-line arguments for FFmpeg
  - `InputData`: Optional data to write to stdin
  - `Executor`: Optional custom command executor

**`ProcessError`** (`process.go`): Error with FFmpeg stderr output
- Fields:
  - `Err`: The underlying error
  - `Stderr`: The captured stderr output

**`RestartInfo`** (`process.go`): Tracks information about process restarts
- Fields:
  - `count`: Number of restart attempts
  - `lastTime`: Timestamp of the last restart attempt

**`BoundedBuffer`** (`process.go`): Thread-safe buffer with size limits
- Methods:
  - `Write(p []byte) (n int, err error)`: Implements io.Writer
  - `String() string`: Returns buffer contents as string
  - `Reset()`: Clears the buffer

#### Command Building

**`StreamFormat`** (`command.go`): Defines audio format parameters
- Fields:
  - `SampleRate`: Sample rate in Hz (e.g., 44100, 48000)
  - `Channels`: Number of audio channels
  - `BitDepth`: Bit depth (e.g., 16, 24, 32)

**`StreamProtocol`** (`command.go`): Enum for stream protocols
- Values:
  - `ProtocolTCP`: TCP transport
  - `ProtocolUDP`: UDP transport
  - `ProtocolHTTP`: HTTP transport
  - `ProtocolHLS`: HLS streaming

**`StreamSource`** (`command.go`): Defines a stream source
- Fields:
  - `URL`: The stream URL
  - `Protocol`: The transport protocol

**`CommandBuilder`** (`command.go`): Fluent interface for building FFmpeg commands
- Fields:
  - Internal fields for building command arguments

#### Export

**`ExportFormat`** (`export.go`): Format settings for audio export
- Fields:
  - `Type`: Format type (e.g., "mp3", "flac", "aac")
  - `Bitrate`: Bitrate setting (e.g., "192k")

**`ExportOptions`** (`export.go`): Options for audio export
- Fields:
  - `FFmpegPath`: Path to FFmpeg executable
  - `Format`: The export format
  - `StreamFormat`: The audio format
  - `OutputPath`: Path to the output file
  - `Timeout`: Maximum time to wait for export

#### Process Monitoring

**`ProcessInfo`** (`monitor.go`): Information about a system process
- Fields:
  - `PID`: Process ID
  - `Name`: Process name

**`ProcessTracker`** (`monitor.go`): Interface for tracking FFmpeg processes
- Methods:
  - `RegisterProcess(url string, process *Process)`: Registers a process
  - `UnregisterProcess(url string)`: Removes a process registration
  - `GetProcess(url string) (*Process, bool)`: Gets a process by URL
  - `ListProcesses() map[string]*Process`: Lists all processes

**`DefaultProcessTracker`** (`monitor.go`): Implementation of `ProcessTracker`
- Fields:
  - `processes`: Thread-safe map of processes

**`SystemProcessFinder`** (`monitor.go`): Interface for finding and managing system processes
- Methods:
  - `FindFFmpegProcesses() ([]ProcessInfo, error)`: Finds FFmpeg processes
  - `IsProcessRunning(pid int) bool`: Checks if a process is running
  - `TerminateProcess(pid int) error`: Terminates a process

**`DefaultSystemProcessFinder`** (`monitor.go`): Implementation of `SystemProcessFinder`

**`Monitor`** (`monitor.go`): Monitors and manages FFmpeg processes
- Fields:
  - Internal fields for tracking processes and configuration

**`MonitorOptions`** (`monitor.go`): Options for creating a monitor
- Fields:
  - `Interval`: Monitoring interval
  - `ProcessFinder`: Process finder implementation
  - `ProcessTracker`: Process tracker implementation

#### Command Execution

**`CommandExecutor`** (`executor.go`): Interface for creating commands
- Methods:
  - `Command(name string, args ...string) Commander`: Creates a command

**`Commander`** (`executor.go`): Interface abstracting exec.Cmd functionality
- Methods:
  - `Start() error`: Starts the command
  - `Wait() error`: Waits for command completion
  - `StdoutPipe() (io.ReadCloser, error)`: Gets stdout pipe
  - `StderrPipe() (io.ReadCloser, error)`: Gets stderr pipe
  - `StdinPipe() (io.WriteCloser, error)`: Gets stdin pipe
  - `SetSysProcAttr(attr *syscall.SysProcAttr)`: Sets process attributes
  - `Process() *os.Process`: Gets the underlying process

**`DefaultCommandExecutor`** (`executor.go`): Standard implementation of `CommandExecutor`

**`DefaultCommander`** (`executor.go`): Implementation of `Commander` wrapping exec.Cmd

### Functions

#### Process Management

**`NewBoundedBuffer(size int) *BoundedBuffer`** (`process.go`): Creates a new bounded buffer
- Arguments:
  - `size`: The maximum size of the buffer
- Returns:
  - A new bounded buffer instance

**`Start(ctx context.Context, opts *ProcessOptions) (*Process, error)`** (`process.go`): Starts a new FFmpeg process
- Arguments:
  - `ctx`: Context for cancellation
  - `opts`: Process options
- Returns:
  - The process instance and any error

**Platform-specific functions**:
- `setupProcessGroup(cmd Commander)` (`platform_unix.go`/`platform_windows.go`): Sets up process group (platform-specific)
- `killProcessGroup(cmd Commander) error` (`platform_unix.go`/`platform_windows.go`): Kills process group (platform-specific)

#### Process Methods

**`StdoutReader() io.Reader`** (`process.go`): Returns a reader for stdout

**`StderrOutput() string`** (`process.go`): Returns captured stderr output

**`Stop() error`** (`process.go`): Stops the FFmpeg process

**`collectStderr()`** (`process.go`): Reads stderr output and stores it in buffer

**`updateRestartInfo()`** (`process.go`): Updates process restart tracking information

**`getRestartDelay() time.Duration`** (`process.go`): Calculates delay for next restart attempt

#### Command Building

**`NewCommandBuilder(ffmpegPath string) *CommandBuilder`** (`command.go`): Creates a new command builder
- Arguments:
  - `ffmpegPath`: Path to FFmpeg executable
- Returns:
  - New CommandBuilder instance

**`getFormatForBitDepth(bitDepth int) string`** (`command.go`): Returns FFmpeg format for bit depth
- Arguments:
  - `bitDepth`: Bit depth value (16, 24, 32)
- Returns:
  - FFmpeg format string (e.g., "s16le")

**`containsOption(args []string, option string) bool`** (`command.go`): Checks if an option exists in arguments
- Arguments:
  - `args`: Argument list
  - `option`: Option to check for
- Returns:
  - Whether option exists

#### CommandBuilder Methods

**`WithFormat(format StreamFormat) *CommandBuilder`** (`command.go`): Sets audio format

**`WithInputURL(source StreamSource) *CommandBuilder`** (`command.go`): Sets input URL

**`WithInputFile(filePath string) *CommandBuilder`** (`command.go`): Sets input file

**`WithInputPipe() *CommandBuilder`** (`command.go`): Configures input from stdin

**`WithOutputPipe() *CommandBuilder`** (`command.go`): Configures output to stdout

**`WithOutputFile(filePath string) *CommandBuilder`** (`command.go`): Sets output file

**`WithOutputFormat(format string) *CommandBuilder`** (`command.go`): Sets output format

**`WithOutputCodec(codec string) *CommandBuilder`** (`command.go`): Sets output audio codec

**`WithOutputBitrate(bitrate string) *CommandBuilder`** (`command.go`): Sets output bitrate

**`WithCustomOption(name, value string) *CommandBuilder`** (`command.go`): Adds custom option

**`DisableVideo() *CommandBuilder`** (`command.go`): Disables video processing

**`Build() []string`** (`command.go`): Builds complete command arguments

**`BuildCommand() string`** (`command.go`): Returns command as a string (for debugging)

#### Export

**`Export(ctx context.Context, data []byte, opts *ExportOptions) error`** (`export.go`): Exports PCM data to file
- Arguments:
  - `ctx`: Context for cancellation
  - `data`: PCM audio data
  - `opts`: Export options
- Returns:
  - Error if export fails

**`getEncoder(format string) string`** (`export.go`): Gets appropriate codec based on format
- Arguments:
  - `format`: Format type (e.g., "mp3", "flac")
- Returns:
  - Codec name (e.g., "libmp3lame")

**`getOutputFormat(format string) string`** (`export.go`): Gets container format for codec
- Arguments:
  - `format`: Format type
- Returns:
  - Container format (e.g., "mp3", "flac", "mp4")

**`getLimitedBitrate(format, requestedBitrate string) string`** (`export.go`): Limits bitrate based on format
- Arguments:
  - `format`: Format type
  - `requestedBitrate`: Requested bitrate
- Returns:
  - Valid bitrate for format

#### Process Monitoring

**`NewProcessTracker() *DefaultProcessTracker`** (`monitor.go`): Creates a new process tracker

**`IsWindows() bool`** (`monitor.go`): Returns true if running on Windows

**`NewMonitor(opts MonitorOptions) *Monitor`** (`monitor.go`): Creates a new FFmpeg process monitor
- Arguments:
  - `opts`: Monitor options
- Returns:
  - New Monitor instance

#### ProcessTracker Methods

**`RegisterProcess(url string, process *Process)`** (`monitor.go`): Registers a process

**`UnregisterProcess(url string)`** (`monitor.go`): Removes a process registration

**`GetProcess(url string) (*Process, bool)`** (`monitor.go`): Gets a process by URL

**`ListProcesses() map[string]*Process`** (`monitor.go`): Lists all registered processes

#### SystemProcessFinder Methods

**`FindFFmpegProcesses() ([]ProcessInfo, error)`** (`monitor.go`): Finds all FFmpeg processes

**`IsProcessRunning(pid int) bool`** (`monitor.go`): Checks if a process is running

**`TerminateProcess(pid int) error`** (`monitor.go`): Terminates a process

**Platform-specific methods**:
- `findFFmpegProcessesUnix() ([]ProcessInfo, error)` (`monitor.go`): Finds processes on Unix
- `findFFmpegProcessesWindows() ([]ProcessInfo, error)` (`monitor.go`): Finds processes on Windows
- `isProcessRunningUnix(pid int) bool` (`monitor.go`): Checks process on Unix
- `isProcessRunningWindows(pid int) bool` (`monitor.go`): Checks process on Windows
- `terminateProcessUnix(pid int) error` (`monitor.go`): Terminates process on Unix
- `terminateProcessWindows(pid int) error` (`monitor.go`): Terminates process on Windows

#### Monitor Methods

**`Start()`** (`monitor.go`): Starts the monitor

**`Stop()`** (`monitor.go`): Stops the monitor

**`IsRunning() bool`** (`monitor.go`): Checks if monitor is running

**`UpdateURLs(urls []string)`** (`monitor.go`): Updates URLs to monitor

**`RegisterProcess(url string, process *Process)`** (`monitor.go`): Registers a process

**`UnregisterProcess(url string)`** (`monitor.go`): Unregisters a process

**`monitorLoop()`** (`monitor.go`): Main monitoring loop

**`checkProcesses() error`** (`monitor.go`): Checks registered processes

**`checkOrphanedSystemProcesses() error`** (`monitor.go`): Finds and cleans orphaned processes

#### Command Execution Methods

**`Command(name string, args ...string) Commander`** (`executor.go`): Creates a command

**`Start() error`** (`executor.go`): Starts the command

**`Wait() error`** (`executor.go`): Waits for completion

**`StdoutPipe() (io.ReadCloser, error)`** (`executor.go`): Gets stdout pipe

**`StderrPipe() (io.ReadCloser, error)`** (`executor.go`): Gets stderr pipe

**`StdinPipe() (io.WriteCloser, error)`** (`executor.go`): Gets stdin pipe

**`SetSysProcAttr(attr *syscall.SysProcAttr)`** (`executor.go`): Sets process attributes

**`Process() *os.Process`** (`executor.go`): Gets process info

## Related Packages

For a higher-level integration that combines this package with the stream manager, see the [FFmpegStream Package](../ffmpegstream/README.md), which provides a complete solution for streaming audio with FFmpeg.