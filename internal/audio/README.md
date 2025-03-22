# Audio Package

## Overview

The `audio` package provides core functionality for audio processing, capture, and analysis for bird sound recognition in the BirdNET-Go application. It serves as the foundation for interacting with audio hardware, managing buffers, and integrating with the BirdNET machine learning model.

This package is designed to be cross-platform (Linux, macOS, and Windows) and modular, allowing different audio sources (physical devices and network streams) to be processed through the same pipeline.

## Package Structure

The audio system is divided into several subpackages:

### Core (`internal/audio`)
- Defines common interfaces, types, and platform-specific utilities
- Provides cross-platform abstractions for audio processing
- Contains platform detection for OS-specific optimizations

### Capture (`internal/audio/capture`)
- Handles audio device interaction through the miniaudio/malgo library
- Manages device enumeration, initialization, and audio data capture
- See detailed documentation in the [capture README](capture/README.md)

### Buffer (`internal/audio/buffer`)
- Provides buffer management for audio data
- Implements both analysis buffers (for real-time processing) and capture buffers (for recording)
- Handles thread-safe read/write operations for multiple audio sources
- See detailed documentation in the [buffer README](buffer/README.md)

### Model (`internal/audio/model`)
- Manages BirdNET model instances
- Maps audio sources to specific model instances
- Provides a unified interface for analyzing audio with BirdNET
- See detailed documentation in the [model README](model/README.md)

### Stream (`internal/audio/stream`)
- Defines interfaces for stream sources and management
- Implements the stream manager for handling multiple audio streams
- Provides APIs for adding, removing, and listing stream sources
- Delivers audio data and level callbacks for consumers
- Includes real-time audio level calculation and clipping detection
- Features activity monitoring with automatic stream restart
- Supports parallel processing of multiple concurrent streams
- See detailed documentation in the [stream README](stream/README.md)

### FFmpeg (`internal/audio/ffmpeg`)
- Manages FFmpeg process lifecycle (starting, monitoring, stopping)
- Provides a fluent command builder interface for FFmpeg commands
- Implements cross-platform process handling and monitoring
- Supports audio export to various formats
- Handles robust error recovery and resource management
- See detailed documentation in the [ffmpeg README](ffmpeg/README.md)

### FFmpegStream (`internal/audio/ffmpegstream`)
- Integrates the `stream` and `ffmpeg` packages into a complete solution
- Provides a high-level `FFmpegManager` class for simplified stream management
- Maps stream configurations to appropriate FFmpeg commands automatically
- Supports multiple stream protocols (RTSP, HLS, HTTP, local files)
- Handles process registration with the monitor for orphaned process cleanup
- Implements clean resource management with proper lifecycle hooks
- Offers audio export functionality for recording capabilities
- Provides a simplified, thread-safe API for applications
- See detailed documentation in the [ffmpegstream README](ffmpegstream/README.md)

## Key Components

### Interfaces
The package defines several key interfaces that form the backbone of the audio system:

- **AudioContext**: Interface for managing audio contexts and devices
- **AudioDevice**: Interface for audio capture devices
- **BufferManager**: Interface for managing audio data buffers
- **DeviceManager**: Interface for managing audio capture devices
- **Processor**: Interface for processing audio data
- **ModelManager**: Interface for managing BirdNET model instances
- **StreamManager**: Interface for managing media streams with callbacks for data and level information
- **Source**: Interface for audio stream sources with lifecycle management methods
- **ProcessTracker**: Interface for tracking FFmpeg processes
- **DataCallback**: Function type for receiving audio data from streams
- **LevelCallback**: Function type for receiving audio level information

### Types
Common types used throughout the audio system:

- **DeviceInfo**: Information about audio devices
- **StreamInfo**: Information about media streams
- **SourceInfo**: Unified information about audio sources (devices or streams)
- **AudioLevelData**: Audio level information for UI feedback (level, clipping state, source)
- **StreamFormat**: Audio format for streams (sample rate, channels, bit depth)
- **AudioFormat**: Format information for audio processing
- **Config**: Configuration for stream sources (URL, transport, inactive timeout, etc.)

### Platform Utilities
Utilities for cross-platform compatibility:

- **GetPlatformDefaultBackend**: Returns the appropriate audio backend for each OS
- **Platform detection functions**: IsLinuxOS, IsWindowsOS, IsMacOS, IsUnixOS

## Data Flow

1. **Audio Source** → Raw audio data is captured from devices or streams
2. **Buffer Manager** → Data is temporarily stored in analysis and capture buffers
3. **Audio Processor** → Audio is processed (filtered, converted, etc.)
4. **Model Manager** → Processed audio is sent to BirdNET for analysis
5. **Application Logic** → Analysis results are used by the application

## Thread Safety

All components implement proper mutex locking to ensure thread-safe operations in concurrent environments:
- Buffer access is protected by read/write mutexes
- Model instances are safely accessed across threads
- Device operations are synchronized
- Stream sources and managers use proper locking for state changes
- FFmpeg process tracking is thread-safe
- Stream processing happens in separate goroutines with proper synchronization

## Error Handling

The package defines common error values and implements proper error wrapping:
- ErrBufferNotFound, ErrBufferExists
- ErrDeviceNotFound, ErrDeviceExists
- ErrStreamNotFound, ErrStreamExists
- ErrSourceNotFound, ErrSourceExists
- Process errors with captured stderr output
- Stream source specific errors ("source already active", "source not found", etc.)

## Cross-Platform Support

The audio package is designed to work across:
- **Linux**: Using ALSA backend for local devices, proper process handling for FFmpeg
- **Windows**: Using WASAPI backend for local devices, tasklist for FFmpeg monitoring
- **macOS**: Using CoreAudio backend for local devices, Unix-style process handling for FFmpeg

## Usage Examples

For detailed usage examples of specific components, please refer to the respective subpackage documentation:

- [Capture Package Documentation](capture/README.md) - For audio device capture
- [Buffer Package Documentation](buffer/README.md) - For audio buffer management
- [Model Package Documentation](model/README.md) - For BirdNET model integration
- [FFmpeg Package Documentation](ffmpeg/README.md) - For FFmpeg process management and streaming
- [Stream Package Documentation](stream/README.md) - For stream source management
- [FFmpegStream Package Documentation](ffmpegstream/README.md) - For integrated FFmpeg streaming

### Quick Start with FFmpegStream

```go
// Create FFmpeg manager with path to FFmpeg executable
manager := ffmpegstream.NewFFmpegManager("/usr/bin/ffmpeg")

// Set callbacks for audio data and levels
manager.SetCallbacks(
    // Process audio data
    func(sourceID string, data []byte) {
        // Process audio data (e.g., send to buffer or analyzer)
    },
    // Handle audio levels
    func(level stream.AudioLevelData) {
        // Update UI with audio levels
        fmt.Printf("Audio level: %d%% for %s\n", level.Level, level.Source)
    },
)

// Start the manager
ctx := context.Background()
manager.Start(ctx)

// Add an RTSP stream
manager.AddStream(stream.Config{
    ID:        "camera1",
    Name:      "Security Camera",
    URL:       "rtsp://example.com/stream1",
    Transport: "tcp",
    Format: stream.AudioFormat{
        SampleRate: 48000,
        Channels:   1,
        BitDepth:   16,
    },
})

// When finished
manager.Stop()
```

## Key Features

- **Device and Stream Support**: Unified handling of both local devices and network streams
- **Cross-Platform Compatibility**: Works on Linux, macOS, and Windows
- **Modular Design**: Components can be used independently or together
- **Thread Safety**: All operations are thread-safe for concurrent use
- **Resource Management**: Proper cleanup of all resources
- **Error Handling**: Consistent error handling patterns
- **Audio Analysis**: Real-time audio level calculation and clipping detection
- **Stream Monitoring**: Automatic recovery of inactive streams
- **Context-Based Cancellation**: Clean shutdown of all operations
- **Protocol Support**: RTSP, HLS, HTTP streaming, and local file support
- **Audio Export**: Convert PCM data to various formats (MP3, FLAC, AAC, etc.)

## Dependencies

- **github.com/gen2brain/malgo**: Cross-platform audio library
- **github.com/smallnest/ringbuffer**: Efficient ring buffer implementation
- **github.com/tphakala/birdnet-go/internal/birdnet**: BirdNET model integration
- **github.com/tphakala/birdnet-go/internal/conf**: Application configuration
- **External dependency**: FFmpeg executable for stream and audio processing 