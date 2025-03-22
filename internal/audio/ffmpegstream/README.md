# FFmpegStream Package

## Overview

The `ffmpegstream` package provides an integrated solution for managing FFmpeg-based audio streams. It serves as a bridge between the high-level stream management interfaces defined in the `stream` package and the low-level FFmpeg process handling provided by the `ffmpeg` package.

This integration layer simplifies the use of FFmpeg for audio streaming by providing a complete, ready-to-use solution that handles the complexity of FFmpeg command building, process management, and stream monitoring.

## Key Components

### `FFmpegManager`

The `FFmpegManager` is the primary component of this package:

- Implements a full-featured stream manager using FFmpeg
- Manages the FFmpeg process monitor for orphaned process cleanup (with 30s default interval)
- Maps stream configurations to appropriate FFmpeg commands
- Maintains proper process registration and lifecycle management
- Provides clean, thread-safe operations through mutex protection
- Implements stream protocol detection (TCP, UDP, HLS, HTTP)
- Supports audio export functionality with configurable formats

#### Methods

- `NewFFmpegManager(ffmpegPath string)`: Creates a new manager with the given FFmpeg executable path
- `Start(ctx context.Context)`: Starts the manager and monitor with proper context handling
- `Stop()`: Stops the manager and all associated streams
- `SetCallbacks(onData, onLevel)`: Sets data and audio level callbacks
- `AddStream(config)`: Adds a new stream with automatic FFmpeg command building
- `RemoveStream(sourceID)`: Removes a stream and cleans up resources
- `GetStream(sourceID)`: Returns a stream by ID
- `ListStreams()`: Lists all managed streams
- `ExportAudio(ctx, data, outputPath, format, streamFormat)`: Exports PCM audio data to various formats

### `processReadCloser`

An internal utility that:

- Wraps FFmpeg process stdout readers with proper close handling
- Ensures process cleanup when streams are closed
- Manages proper unregistration of processes from the monitor
- Implements io.ReadCloser interface for seamless stream integration

## Features

- **Easy Stream Management**: Add, remove, and list streams with simple API calls
- **Automatic Process Monitoring**: Orphaned processes are detected and cleaned up
- **Stream Health Monitoring**: Inactive streams are detected and restarted
- **Clean Resource Management**: Processes are properly terminated when streams are closed
- **Cross-Platform Support**: Works on Linux, macOS, and Windows
- **Audio Export**: Convert PCM data to various audio formats
- **Protocol Support**: Automatic detection and configuration for RTSP, HLS, HTTP
- **Stream Lifecycle Control**: Start, stop, and monitor streams with clean error handling

## Usage Example

```go
// Create an FFmpeg manager
manager := ffmpegstream.NewFFmpegManager("/usr/bin/ffmpeg")

// Set up callbacks for stream data
manager.SetCallbacks(
    // Data callback
    func(sourceID string, data []byte) {
        // Process raw audio data
        fmt.Printf("Received %d bytes from %s\n", len(data), sourceID)
    },
    // Audio level callback
    func(level stream.AudioLevelData) {
        // Update UI with audio level
        fmt.Printf("Audio level: %d%% for %s\n", level.Level, level.Name)
    },
)

// Start the manager
ctx := context.Background()
err := manager.Start(ctx)
if err != nil {
    log.Fatalf("Failed to start manager: %v", err)
}

// Add an RTSP stream
rtspConfig := stream.Config{
    ID:        "camera1",
    Name:      "Security Camera",
    URL:       "rtsp://example.com/stream1",
    Transport: "tcp",
    Format: stream.AudioFormat{
        SampleRate: 48000,
        Channels:   1,
        BitDepth:   16,
    },
    InactiveTime: 60 * time.Second,
}

err = manager.AddStream(rtspConfig)
if err != nil {
    log.Fatalf("Failed to add RTSP stream: %v", err)
}

// Add an HLS stream
hlsConfig := stream.Config{
    ID:        "webstream",
    Name:      "Web Radio",
    URL:       "http://example.com/stream.m3u8",
    Transport: "hls",
    Format: stream.AudioFormat{
        SampleRate: 44100,
        Channels:   2,
        BitDepth:   16,
    },
}

err = manager.AddStream(hlsConfig)
if err != nil {
    log.Fatalf("Failed to add HLS stream: %v", err)
}

// Get a list of all streams
streams := manager.ListStreams()
for _, s := range streams {
    fmt.Printf("Stream: %s (%s)\n", s.ID(), s.Name())
}

// Get a specific stream
if stream, err := manager.GetStream("webstream"); err == nil {
    fmt.Printf("Stream active: %v\n", stream.IsActive())
}

// Remove a stream
err = manager.RemoveStream("camera1")
if err != nil {
    log.Printf("Failed to remove stream: %v", err)
}

// Export audio to a file
audioData := []byte{...} // PCM audio data
err = manager.ExportAudio(ctx, audioData, "output.mp3", 
    ffmpeg.ExportFormat{
        Type:    "mp3",
        Bitrate: "192k",
    }, 
    stream.AudioFormat{
        SampleRate: 44100,
        Channels:   2,
        BitDepth:   16,
    },
)
if err != nil {
    log.Printf("Failed to export audio: %v", err)
}

// Shutdown the manager
manager.Stop()
```

## Integration with Other Packages

The `ffmpegstream` package integrates:

- **`stream` package**: Uses the stream interfaces and types for consistent audio stream handling
  - Implements the Source interface with FFmpegSource
  - Uses the Manager interface through DefaultManager
  - Leverages callbacks for data and level reporting
  
- **`ffmpeg` package**: Leverages the FFmpeg process management and command building
  - Uses Monitor for orphaned process cleanup
  - Uses CommandBuilder for creating FFmpeg commands
  - Uses Process for managing FFmpeg process lifecycle
  - Uses Export for audio file conversion

This provides a complete solution that combines high-level stream abstractions with robust FFmpeg process handling.

## How It Works

1. When `AddStream` is called, the manager:
   - Creates a process starter function that configures and starts an FFmpeg process
   - Maps the stream config to FFmpeg-specific options (protocol, format, etc.)
   - Creates a stream source using the process starter
   - Adds the source to the stream manager

2. When `Start` is called:
   - The FFmpeg monitor starts scanning for orphaned processes
   - The stream manager starts, activating all registered streams
   - Each stream source starts its FFmpeg process
   - Each stream's output is processed for data and level callbacks

3. When a stream is closed or removed:
   - The FFmpeg process is properly stopped
   - The process is unregistered from the monitor
   - Resources are cleaned up properly

## Thread Safety

The package ensures thread-safe operations:

- All manager methods are protected by appropriate mutex locks
- Process registration and unregistration are properly synchronized
- State changes are atomic and safely handled
- Access to shared resources is protected

## Error Handling

Comprehensive error handling includes:

1. All errors from underlying packages are properly propagated
2. Additional context is added to errors for better diagnostics
3. Process errors include stderr output for easier debugging
4. Stream errors are handled gracefully without affecting other streams

## Supported Stream Sources

The `FFmpegManager` supports various stream protocols:

- **RTSP**: Real-Time Streaming Protocol (TCP or UDP)
- **HLS**: HTTP Live Streaming
- **HTTP**: Direct HTTP streams
- **Local files**: Local audio files as streams

## Performance Considerations

- Process monitoring interval is configurable (defaults to 30 seconds)
- Stream inactivity detection is tunable (defaults to 60 seconds)
- Resource usage is minimized by proper process management
- Proper cleanup ensures no resource leaks
- Output handling is optimized with appropriate buffer sizes 