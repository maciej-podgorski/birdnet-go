# Audio Package

## Overview

The `audio` package provides core functionality for audio processing, capture, and analysis for bird sound recognition in the BirdNet-Go application. It serves as the foundation for interacting with audio hardware, managing buffers, and integrating with the BirdNET machine learning model.

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

### Model (`internal/audio/model`)
- Manages BirdNET model instances
- Maps audio sources to specific model instances
- Provides a unified interface for analyzing audio with BirdNET

## Key Components

### Interfaces
The package defines several key interfaces that form the backbone of the audio system:

- **AudioContext**: Interface for managing audio contexts and devices
- **AudioDevice**: Interface for audio capture devices
- **BufferManager**: Interface for managing audio data buffers
- **DeviceManager**: Interface for managing audio capture devices
- **Processor**: Interface for processing audio data
- **ModelManager**: Interface for managing BirdNET model instances
- **StreamManager**: Interface for managing media streams

### Types
Common types used throughout the audio system:

- **DeviceInfo**: Information about audio devices
- **StreamInfo**: Information about media streams
- **SourceInfo**: Unified information about audio sources (devices or streams)
- **AudioLevelData**: Audio level information for UI feedback

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

## Error Handling

The package defines common error values and implements proper error wrapping:
- ErrBufferNotFound, ErrBufferExists
- ErrDeviceNotFound, ErrDeviceExists
- ErrStreamNotFound, ErrStreamExists
- ErrSourceNotFound, ErrSourceExists

## Cross-Platform Support

The audio package is designed to work across:
- **Linux**: Using ALSA backend
- **Windows**: Using WASAPI backend
- **macOS**: Using CoreAudio backend

## Usage Examples

For detailed usage examples of specific components, please refer to the respective subpackage documentation:

- [Capture Package Documentation](capture/README.md) - For audio device capture
- Buffer Package - For audio buffer management
- Model Package - For BirdNET model integration

## Dependencies

- **github.com/gen2brain/malgo**: Cross-platform audio library
- **github.com/smallnest/ringbuffer**: Efficient ring buffer implementation
- **github.com/tphakala/birdnet-go/internal/birdnet**: BirdNET model integration
- **github.com/tphakala/birdnet-go/internal/conf**: Application configuration 