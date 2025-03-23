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
- Implements direct integrations for FFmpeg streaming functionality via `streaming.go`
- Offers simple factory functions for creating and connecting audio components

### Capture (`internal/audio/capture`)
- Handles audio device interaction through the miniaudio/malgo library
- Manages device enumeration, initialization, and audio data capture
- See detailed documentation in the [capture README](capture/README.md)

### Buffer (`internal/audio/buffer`)
- Provides thread-safe buffer management for audio data
- Implements both analysis buffers (for real-time processing) and capture buffers (for recording)
- Handles consistent read/write operations for multiple audio sources
- Replaces legacy functionality from the myaudio package with a modern interface-based design
- Facilitates dependency injection for improved testability
- Features comprehensive error handling and robust resource management
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

## System Architecture

The audio system follows a layered architecture with clear component responsibilities:

```
┌────────────────────────────────────────────────────────────┐
│                        Application                         │
└───────────────────────────┬────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│                     Audio Controller                       │
│                                                            │
│  ┌─────────────────┐   ┌─────────────────┐   ┌──────────┐  │
│  │  Device Manager │   │ Stream Manager  │   │ Processor│  │
│  └────────┬────────┘   └────────┬────────┘   └────┬─────┘  │
└───────────┼─────────────────────┼─────────────────┼────────┘
            │                     │                 │
┌───────────▼─────────────────────▼─────────────────▼────────┐
│                       Buffer Manager                       │
│                                                            │
│  ┌─────────────────┐   ┌─────────────────┐                 │
│  │ Analysis Buffers│   │ Capture Buffers │                 │
│  └─────────────────┘   └─────────────────┘                 │
└────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│                       Model Manager                        │
│                                                            │
│  ┌─────────────────┐   ┌─────────────────┐                 │
│  │  BirdNET Model  │   │  Custom Models  │                 │
│  └─────────────────┘   └─────────────────┘                 │
└────────────────────────────────────────────────────────────┘
```

### Audio Flow

1. **Audio Sources** → Raw audio from devices (via Capture) or streams (via FFmpegStream)
2. **Buffer Manager** → Thread-safe storage in Analysis and Capture buffers
3. **Model Manager** → Audio processing and analysis using BirdNET
4. **Application Logic** → Bird detection results handling

## Key Components

### Streaming Components
The audio package provides direct implementations of streaming interfaces:

- **FFmpegStreamManager**: Implements the StreamManager interface using the ffmpegstream package
  - Handles stream creation, management, and callbacks
  - Provides methods for starting/stopping streams and monitoring activity
  - Integrates with FFmpeg process monitoring automatically

- **FFmpegMonitor**: Implements the FFmpegMonitorInterface using the ffmpeg package
  - Monitors and manages FFmpeg processes
  - Prevents orphaned processes that could consume resources

- **CreateAudioComponents**: Factory function that creates and initializes both components at once
  - Returns ready-to-use streaming and monitoring components
  - Simplifies application setup with a single function call

### Interfaces
The package defines several key interfaces that form the backbone of the audio system:

- **AudioContext**: Interface for managing audio contexts and devices
- **AudioDevice**: Interface for audio capture devices
- **BufferManager**: Interface for managing audio data buffers
- **DeviceManager**: Interface for managing audio capture devices
- **Processor**: Interface for processing audio data
- **ModelManager**: Interface for managing BirdNET model instances
- **StreamManager**: Interface for managing media streams with callbacks for data and level information
- **FFmpegMonitorInterface**: Interface for FFmpeg process monitoring
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

## Legacy vs. Modern Audio Components

The audio system has undergone significant architectural improvements, moving from global functions to a structured, interface-based approach:

### Legacy Components (Deprecated)

The `myaudio` package (being gradually replaced):
- Uses global variables and functions
- Lacks proper dependency management
- Has limited thread safety
- Provides basic buffer functionality without clear separation of concerns

### Modern Components (Recommended)

The structured audio package hierarchy:
- Uses interfaces for clean dependency injection
- Implements proper concurrency protection
- Features comprehensive error handling
- Follows SOLID design principles
- Enables better testability through mock implementations
- Provides cleaner API boundaries between components

### Migration Path

Projects should transition from the legacy `myaudio` package to the modern audio components:

1. Replace global function calls with BufferManager methods
2. Inject dependencies rather than using global state
3. Use the component factories to create properly connected instances
4. Follow the usage examples in each subpackage README

See the [Buffer README](buffer/README.md) for detailed migration steps.

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

## Dependencies

The audio system has the following key dependencies:

### External Libraries
- **github.com/gen2brain/malgo**: Cross-platform audio library for device capture
- **github.com/smallnest/ringbuffer**: Efficient ring buffer implementation for audio analysis buffers
- **FFmpeg executable**: Required for stream processing and audio format conversion

### Internal Dependencies
- **github.com/tphakala/birdnet-go/internal/birdnet**: BirdNET model integration
- **github.com/tphakala/birdnet-go/internal/conf**: Application configuration

### Standard Library
- **os/exec**: For process management (FFmpeg)
- **sync**: For concurrency control (mutex, waitgroup)
- **time**: For timing and timeout operations
- **context**: For cancellation and deadline handling

## Usage Examples

For detailed usage examples of specific components, please refer to the respective subpackage documentation:

- [Capture Package Documentation](capture/README.md) - For audio device capture
- [Buffer Package Documentation](buffer/README.md) - For audio buffer management
- [Model Package Documentation](model/README.md) - For BirdNET model integration
- [FFmpeg Package Documentation](ffmpeg/README.md) - For FFmpeg process management and streaming
- [Stream Package Documentation](stream/README.md) - For stream source management
- [FFmpegStream Package Documentation](ffmpegstream/README.md) - For integrated FFmpeg streaming

### Quick Start with Integrated Streaming Components

```go
// Get FFmpeg executable path
ffmpegPath := "/usr/bin/ffmpeg" // or find dynamically

// Create audio components with a single function call
streamManager, ffmpegMonitor := audio.CreateAudioComponents(ffmpegPath)

// Set callbacks for audio data and levels
streamManager.SetCallbacks(
    // Process audio data
    func(sourceID, sourceName string, data []byte) {
        // Process audio data (e.g., send to buffer or analyzer)
        fmt.Printf("Received %d bytes from %s (%s)\n", len(data), sourceName, sourceID)
    },
    // Handle audio levels
    func(level audio.AudioLevelData) {
        // Update UI with audio levels
        fmt.Printf("Audio level: %d%% for %s\n", level.Level, level.Source)
        if level.Clipping {
            fmt.Println("Warning: Audio clipping detected!")
        }
    },
    // Restart notification
    func() {
        fmt.Println("Stream restart detected")
    },
)

// Start the stream manager
if err := streamManager.Start(); err != nil {
    log.Fatal("Failed to start stream manager:", err)
}

// Add an RTSP stream
if err := streamManager.StartStream("rtsp://example.com/stream1", "tcp"); err != nil {
    log.Printf("Error starting stream: %v", err)
}

// Check if a stream is active
if streamManager.IsStreamActive("rtsp://example.com/stream1") {
    fmt.Println("Stream is active!")
}

// List all active streams
activeStreams := streamManager.ListActiveStreams()
fmt.Println("Active streams:", activeStreams)

// When finished
streamManager.StopAllStreams()
streamManager.Stop()
ffmpegMonitor.Stop()
```

### Complete Audio System Integration

```go
// Initialize the buffer manager
factory := buffer.NewBufferFactory()
bufferManager := factory.CreateBufferManager()

// Initialize the model manager with BirdNET
settings := conf.Settings()
modelManager := model.NewManager(settings)

// Set up audio capture
captureFactory := capture.NewContextFactory(settings.Debug)
audioCtx, _ := captureFactory.CreateAudioContext(logFunc)
captureManager := capture.NewDeviceManager(audioCtx, bufferManager)

// Set up audio streaming
streamManager, ffmpegMonitor := audio.CreateAudioComponents(ffmpegPath)

// Set up data callbacks
captureManager.SetDataCallback(func(sourceID string, data []byte, frameCount uint32) {
    // Send data to buffer manager
    if err := bufferManager.WriteToAnalysisBuffer(sourceID, data); err != nil {
        log.Printf("Error writing to analysis buffer: %v", err)
        return
    }
    
    // Also write to capture buffer for recording
    if err := bufferManager.WriteToCaptureBuffer(sourceID, data); err != nil {
        log.Printf("Error writing to capture buffer: %v", err)
    }
    
    // Analyze audio data
    analysisData, err := bufferManager.ReadFromAnalysisBuffer(sourceID, nil)
    if err != nil || analysisData == nil {
        return // Not enough data yet
    }
    
    // Send to model for analysis
    modelManager.Analyze(sourceID, analysisData, time.Now().Unix())
})

// Start audio capture
captureManager.StartCapture("deviceName", 48000, 1)

// Start a stream
streamManager.StartStream("rtsp://example.com/stream", "tcp")

// Clean up when done
func cleanup() {
    captureManager.Close()
    streamManager.Stop()
    ffmpegMonitor.Stop()
    bufferManager.CleanupAllBuffers()
    modelManager.Cleanup()
}
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
- **Simple Integration**: Direct implementations of audio interfaces using the stream and FFmpeg packages
- **Factory Functions**: Easy creation of connected components with sensible defaults