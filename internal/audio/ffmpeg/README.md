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

## Related Packages

For a higher-level integration that combines this package with the stream manager, see the [FFmpegStream Package](../ffmpegstream/README.md), which provides a complete solution for streaming audio with FFmpeg.