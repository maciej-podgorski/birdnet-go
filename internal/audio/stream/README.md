# Stream Package

## Overview

The `stream` package provides a flexible and extensible framework for managing audio streams from various sources. It defines interfaces and implementations for working with stream sources in a consistent way, regardless of the underlying transport mechanism or audio format.

This package is part of the BirdNET-Go audio system and works in conjunction with the `ffmpeg` package to provide robust stream handling capabilities.

## Key Components

### Interfaces

- `Source`: Represents an audio stream source with methods for lifecycle management (Start, Stop, ID, Name, IsActive)
- `Manager`: Manages multiple stream sources and provides callbacks for data handling (AddSource, RemoveSource, GetSource, ListSources, Start, Stop, SetCallbacks)

### Types

- `AudioLevelData`: Contains audio level information for UI feedback (Level, Clipping, Source, Name)
- `AudioFormat`: Defines audio format parameters (SampleRate, Channels, BitDepth, Format)
- `Config`: Configuration for stream sources including transport options (ID, Name, URL, Format, Transport, BufferSize, InactiveTime)

### Callbacks

- `DataCallback`: Function type for receiving audio data from streams `func(sourceID string, data []byte)`
- `LevelCallback`: Function type for receiving audio level information `func(level AudioLevelData)`

## Implementations

### `DefaultManager`

The `DefaultManager` provides a complete implementation of the `Manager` interface:

- Thread-safe management of multiple stream sources through read/write mutexes
- Proper lifecycle management with Start/Stop methods
- Efficient callback delivery for audio data and levels
- Clean error handling and recovery
- Context-based cancellation for graceful shutdown
- Parallel processing of multiple streams with proper resource cleanup

### `FFmpegSource`

The `FFmpegSource` implements the `Source` interface using FFmpeg as the underlying processor:

- Robust process management with proper cleanup
- Watchdog mechanism for detecting inactive streams (defaulting to 60s inactivity timeout)
- Automatic stream restart on inactivity detected with a 5s check interval
- Thread-safe operations through mutex protection
- Dependency injection through the FFmpegProcessStarter function type

### `WatchedReader`

A wrapper around `io.ReadCloser` that tracks data flow:

- Notifies on data reads for inactivity detection
- Properly closes underlying reader when closed
- Maintains the original reader interface while adding monitoring functionality

## Stream Processing

The package includes audio level calculation functionality:

- Raw PCM data is analyzed to determine audio levels
- Audio levels are scaled to a 0-100 percentage for easy display
- Clipping detection is implemented to identify potentially distorted audio
- Level information includes source identification

## Usage Example

```go
// Create a stream manager
manager := stream.NewManager()

// Set up callbacks for stream data
manager.SetCallbacks(
    // Data callback
    func(sourceID string, data []byte) {
        fmt.Printf("Received %d bytes from %s\n", len(data), sourceID)
    },
    // Audio level callback
    func(level stream.AudioLevelData) {
        fmt.Printf("Audio level: %d%% for %s\n", level.Level, level.Source)
        if level.Clipping {
            fmt.Printf("Warning: Audio clipping detected on %s\n", level.Name)
        }
    },
)

// Create a source
sourceConfig := stream.Config{
    ID:           "camera1",
    Name:         "Front Camera",
    URL:          "rtsp://example.com/stream1",
    Transport:    "tcp",
    Format: stream.AudioFormat{
        SampleRate: 48000,
        Channels:   1,
        BitDepth:   16,
        Format:     "s16le",
    },
    BufferSize:   32768,
    InactiveTime: 60 * time.Second,
}

// Create a process starter function
processStarter := func(ctx context.Context, cfg stream.Config) (io.ReadCloser, error) {
    // Implementation specific to your audio source
    // This is where you'd connect to your stream source
    return someReader, nil
}

// Create the source
source := stream.NewFFmpegSource(sourceConfig, processStarter)

// Add the source to the manager
err := manager.AddSource(source)
if err != nil {
    log.Fatalf("Failed to add source: %v", err)
}

// Start the manager and all sources
ctx := context.Background()
err = manager.Start(ctx)
if err != nil {
    log.Fatalf("Failed to start manager: %v", err)
}

// Later, retrieve a source by ID
if source, err := manager.GetSource("camera1"); err == nil {
    fmt.Printf("Source active: %v\n", source.IsActive())
}

// List all managed sources
sources := manager.ListSources()
for _, s := range sources {
    fmt.Printf("Source: %s (%s)\n", s.ID(), s.Name())
}

// ... Later ...
manager.Stop()
```

## Thread Safety

All components in the stream package implement proper mutex locking to ensure thread-safe operations in concurrent environments:

- Source implementations use mutexes to protect state changes
- Manager implementations use read/write mutexes for source management
- Callbacks are delivered in a way that prevents deadlocks
- Stream processing happens in separate goroutines with proper synchronization

## Error Handling

The package follows consistent error handling patterns:

1. All errors are wrapped with context using `fmt.Errorf` and `%w`
2. Methods return specific errors for common failure scenarios:
   - "source already active"
   - "source with ID X already exists"
   - "source with ID X not found"
   - "manager already running"
3. Errors are isolated to prevent cascading failures
4. Process isolation ensures that failures in one stream don't affect others

## Integration with FFmpeg

While this package defines generic stream interfaces, it's designed to work seamlessly with the `ffmpeg` package:

- `FFmpegSource` provides a bridge between stream interfaces and FFmpeg processes
- Stream source configurations map directly to FFmpeg command builder options

### FFmpegStream Integration

For a complete, pre-built integration between this package and the FFmpeg package, see the [FFmpegStream Package](../ffmpegstream/README.md), which provides:

- Ready-to-use `FFmpegManager` that implements all the integration code
- Automatic FFmpeg process management and monitoring
- Simplified API for working with FFmpeg-based streams
- Support for all major streaming protocols (RTSP, HLS, HTTP)
- Built-in audio export functionality

The FFmpegStream package is the recommended way to use this package with FFmpeg streams as it handles all the complexity of:
- Creating appropriate FFmpeg commands
- Managing process lifecycle
- Handling proper cleanup
- Protocol-specific configurations
- Watchdog and monitoring functionality

## Features

- Support for multiple simultaneous streams with parallel processing
- Stream activity monitoring with automatic recovery
- Clean resource management including proper goroutine cleanup
- Consistent audio data delivery regardless of source
- Audio level calculation for UI feedback with clipping detection
- Configurable buffer sizes and inactivity timeouts
- Context-based cancellation for clean shutdown
- Extensible design allowing for different stream source implementations 