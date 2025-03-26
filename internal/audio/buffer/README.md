# Audio Buffer Package

## Overview

The `buffer` package provides efficient audio buffer management for the BirdNET-Go application. It implements thread-safe ring and circular buffers optimized for audio data processing, supporting both real-time analysis and audio capture functionality. This package is designed to work seamlessly across different operating systems (Linux, macOS, and Windows).

This package is part of the larger [audio system](../README.md) which provides comprehensive audio processing capabilities for the BirdNet-Go application.

## Key Components

### Core Interfaces

- **AudioBuffer**: Base interface defining common operations for all buffer types
  - Provides fundamental read/write operations
  - Handles buffer reset functionality
  - Maintains sample rate and channel information
  - **Important**: Designed for single audio channel per buffer

- **AnalysisBufferInterface**: Interface for analysis-specific buffer operations
  - Extends AudioBuffer with analysis-specific functionality
  - Provides methods for checking analysis readiness
  - **Note**: Each buffer handles exactly one audio channel

- **CaptureBufferInterface**: Interface for capture-specific buffer operations
  - Extends AudioBuffer with capture-specific functionality
  - Enables extraction of time-based audio segments
  - **Note**: Each buffer handles exactly one audio channel

- **BufferManagerInterface**: Interface for buffer management operations
  - Defines methods for buffer allocation and deallocation
  - Provides operations for reading from and writing to buffers
  - Handles buffer cleanup and resource management

- **RingBufferInterface**: Interface for abstracting ring buffer operations
  - Defines core read/write operations
  - Provides buffer state methods (Length, Capacity, Free)
  - Enables buffer reset functionality
  - Allows for dependency injection of different ring buffer implementations

### Implementation Classes

- **AnalysisBuffer**: Implements the AnalysisBufferInterface
  - Uses a ring buffer for efficient memory usage
  - Supports overlapping audio segments for continuous analysis
  - Provides thread-safe operations with proper mutex locks
  - Implements automatic overflow detection and warning mechanisms
  - Handles partial reads and writes with proper error handling
  - Uses configurable overlap fraction for analysis window sliding

- **CaptureBuffer**: Implements the CaptureBufferInterface
  - Circular buffer for storing timestamped PCM audio data
  - Maintains time-to-buffer mapping for accurate audio segment retrieval
  - Handles buffer wrap-around with proper timestamp adjustments
  - Thread-safe with mutex protection for concurrent access
  - Aligns buffer size to optimize memory access patterns (2KB boundaries)
  - Provides time-based segment extraction with timeout handling

- **BufferManager**: Implements the BufferManagerInterface
  - Central component managing multiple buffers
  - Maps buffers to specific audio sources (devices or streams)
  - Provides unified interface for read/write operations
  - Ensures thread safety with separate mutex locks for analysis and capture operations
  - Validates inputs for all operations with descriptive error messages
  - Handles resource cleanup with proper error propagation

### Supporting Components

- **BufferFactory**: Creates properly configured buffer instances
  - Allows dependency injection for testing and flexibility
  - Creates analysis buffers, capture buffers, and buffer managers
  - Configures buffer instances with appropriate dependencies
  - Provides default and customizable factory methods

- **BufferConfig**: Contains configuration parameters for buffers
  - Stores BirdNET-specific settings like overlap fraction
  - Defines sample rate, bit depth, and buffer size parameters
  - Provides default values for buffer durations
  - Centralizes configuration for consistent buffer behavior

- **Dependencies**: Abstractions for external dependencies
  - TimeProvider: Abstracts time-related operations for better testability
  - Logger: Provides logging interface for consistent error and status reporting
  - Both interfaces have real implementations (RealTimeProvider, StandardLogger)

## Core Operations

### Analysis Buffer Operations

```go
// Create a buffer factory with default settings
factory := NewBufferFactory()

// Create an analysis buffer (3 seconds duration, 48kHz sample rate, mono)
buffer := factory.CreateAnalysisBuffer(48000, 1, 3*time.Second)

// Write audio data to the buffer
bytesWritten, err := buffer.Write(audioData)

// Read overlapping segments for analysis (analysis window size in bytes)
audioSegment := make([]byte, analysisWindowSize)
bytesRead, err := buffer.Read(audioSegment)

// Check if enough data is available for analysis
if buffer.ReadyForAnalysis() {
    // Perform analysis
}

// Optionally adjust the overlap between sliding windows
buffer.SetOverlapFraction(0.5) // 50% overlap
```

### Capture Buffer Operations

```go
// Create a buffer factory with default settings
factory := NewBufferFactory()

// Create a capture buffer (30 seconds duration, 44.1kHz sample rate, mono)
buffer := factory.CreateCaptureBuffer(44100, 1, 30*time.Second)

// Write audio data to the buffer
bytesWritten, err := buffer.Write(audioData)

// Extract a 5-second audio segment starting at a specific time
startTime := time.Now().Add(-10 * time.Second)
audioSegment, err := buffer.ReadSegment(startTime, 5)
```

### Buffer Manager Usage

```go
// Create a buffer factory and buffer manager
factory := NewBufferFactory()
manager := factory.CreateBufferManager()

// Allocate buffers for a specific audio source
sourceID := "microphone1"
err := manager.AllocateAnalysisBuffer(bufferCapacity, sourceID)
err = manager.AllocateCaptureBuffer(30, 48000, 2, sourceID)

// Write audio data to the buffers
err = manager.WriteToAnalysisBuffer(sourceID, audioData)
err = manager.WriteToCaptureBuffer(sourceID, audioData)

// Read data from the analysis buffer
analysisData, err := manager.ReadFromAnalysisBuffer(sourceID, nil)

// Extract a segment from the capture buffer
startTime := time.Now().Add(-5 * time.Second)
captureSegment, err := manager.ReadSegmentFromCaptureBuffer(sourceID, startTime, 3)

// Check if buffers exist for a source
hasAnalysis := manager.HasAnalysisBuffer(sourceID)
hasCapture := manager.HasCaptureBuffer(sourceID)

// Clean up resources
manager.CleanupAllBuffers()
```

## Migrating from Legacy myaudio Package

The `buffer` package replaces the legacy `myaudio` package for audio buffer management with several key improvements:

### Key Improvements

1. **Interface-Based Design**: Buffer operations are defined by interfaces like `BufferManagerInterface`, enabling dependency injection and better testability.
2. **Proper Dependency Management**: All dependencies are explicitly passed to buffer components rather than using global state.
3. **Thread Safety**: Comprehensive mutex protection with proper lock granularity for optimal concurrency.
4. **Error Handling**: Consistent error patterns with proper context and wrapping.
5. **Resource Management**: Explicit lifecycle methods for buffer allocation and cleanup.
6. **Standardized Source ID Handling**: Consistent handling of source IDs with or without prefixes.
7. **Simplified API**: More intuitive API with consistent parameter ordering and naming.
8. **Improved Testability**: Mock implementations available for all interfaces to facilitate unit testing.

### Migration Steps

To migrate from the legacy myaudio package to the buffer package:

1. **Initialize Buffer Components**:
   ```go
   // Create a buffer factory and manager
   factory := buffer.NewBufferFactory()
   bufferManager := factory.CreateBufferManager()
   ```

2. **Update Function Calls**:
   
   *Old approach (myaudio):*
   ```go
   // Initialize a buffer
   myaudio.AllocateCaptureBuffer(60, 48000, 2, "deviceID")
   
   // Write to buffer
   myaudio.WriteToCaptureBuffer("deviceID", audioData)
   
   // Read from buffer
   pcmData, err := myaudio.ReadSegmentFromCaptureBuffer("deviceID", startTime, 15)
   
   // Remove buffer when done
   myaudio.RemoveCaptureBuffer("deviceID")
   ```

   *New approach (buffer):*
   ```go
   // Initialize a buffer
   bufferManager.AllocateCaptureBuffer(60, 48000, 2, "deviceID")
   
   // Write to buffer
   bufferManager.WriteToCaptureBuffer("deviceID", audioData)
   
   // Read from buffer
   pcmData, err := bufferManager.ReadSegmentFromCaptureBuffer("deviceID", startTime, 15)
   
   // Remove buffer when done
   bufferManager.RemoveCaptureBuffer("deviceID")
   ```

3. **Add Buffer Manager to Structs**:
   ```go
   type MyComponent struct {
       // ... other fields
       BufferManager buffer.BufferManagerInterface
   }
   
   func (c *MyComponent) Process() {
       // Use c.BufferManager instead of global myaudio functions
   }
   ```

4. **Dependency Injection**:
   ```go
   // Pass buffer manager to components that need it
   proc := processor.New(settings, dataStore, bn, metrics, imageCache, bufferManager)
   ```

5. **Handle Source IDs Consistently**:
   The buffer manager standardizes source IDs internally, so you don't need to worry about prefixes like "device:" anymore:
   ```go
   // Both will work with the new buffer manager
   bufferManager.ReadSegmentFromCaptureBuffer("deviceID", startTime, 15)
   bufferManager.ReadSegmentFromCaptureBuffer("device:deviceID", startTime, 15)
   ```

6. **Update Integration Points**:
   
   *For analysis processing:*
   ```go
   // Old approach using global functions
   func processAudio(sourceID string) {
       // Read data from global analysis buffer
       analysisData, err := myaudio.ReadFromAnalysisBuffer(sourceID, nil)
       if err != nil || analysisData == nil {
           return
       }
       
       // Process the data
       analyzeAudio(analysisData)
   }
   
   // New approach using BufferManager
   func (p *Processor) processAudio(sourceID string) {
       // Read data from buffer manager
       analysisData, err := p.BufferManager.ReadFromAnalysisBuffer(sourceID, nil)
       if err != nil || analysisData == nil {
           return
       }
       
       // Process the data
       p.analyzeAudio(analysisData)
   }
   ```
   
   *For audio export:*
   ```go
   // Old approach using global functions
   func exportAudio(sourceID string, startTime time.Time) ([]byte, error) {
       // Read segment from global capture buffer
       return myaudio.ReadSegmentFromCaptureBuffer(sourceID, startTime, 15)
   }
   
   // New approach using BufferManager
   func (a *AudioExporter) exportAudio(sourceID string, startTime time.Time) ([]byte, error) {
       // Read segment from buffer manager
       return a.BufferManager.ReadSegmentFromCaptureBuffer(sourceID, startTime, 15)
   }
   ```

7. **Update Initialization in Main Application**:
   ```go
   // Create the buffer manager
   factory := buffer.NewBufferFactory()
   bufferManager := factory.CreateBufferManager()
   
   // Pass it to all components that need it
   deviceManager := capture.NewDeviceManager(audioCtx, bufferManager)
   processor := analysis.NewProcessor(settings, bufferManager)
   exporter := audio.NewExporter(bufferManager)
   
   // Start capturing devices with buffers automatically allocated
   deviceManager.StartCapture("myDevice", 48000, 1)
   ```

### Common Migration Patterns

#### Global to Local Transition

**Before:**
```go
package mypackage

import (
    "github.com/tphakala/birdnet-go/internal/myaudio"
)

func processAudioSegment(sourceID string, startTime time.Time) error {
    // Using global functions
    pcmData, err := myaudio.ReadSegmentFromCaptureBuffer(sourceID, startTime, 15)
    if err != nil {
        return err
    }
    
    // Process pcmData
    return nil
}
```

**After:**
```go
package mypackage

import (
    "github.com/tphakala/birdnet-go/internal/audio/buffer"
)

type Processor struct {
    BufferManager buffer.BufferManagerInterface
}

func (p *Processor) processAudioSegment(sourceID string, startTime time.Time) error {
    // Using the injected buffer manager
    pcmData, err := p.BufferManager.ReadSegmentFromCaptureBuffer(sourceID, startTime, 15)
    if err != nil {
        return err
    }
    
    // Process pcmData
    return nil
}
```

#### DatabaseAction Example

A complete example showing the migration of the DatabaseAction component:

**Before:**
```go
package processor

import (
    "time"
    "log"
    
    "github.com/tphakala/birdnet-go/internal/myaudio"
)

type DatabaseAction struct {
    Note *Note
    // other fields
}

func (a *DatabaseAction) Execute() error {
    // Read segment using global function
    pcmData, err := myaudio.ReadSegmentFromCaptureBuffer(a.Note.Source, a.Note.BeginTime, 15)
    if err != nil {
        log.Printf("Error reading PCM data: %v", err)
        return err
    }
    
    // Save audio to database
    return saveAudio(pcmData)
}
```

**After:**
```go
package processor

import (
    "time"
    "log"
    
    "github.com/tphakala/birdnet-go/internal/audio/buffer"
)

type DatabaseAction struct {
    Note *Note
    BufferManager buffer.BufferManagerInterface
    // other fields
}

func (a *DatabaseAction) Execute() error {
    // Read segment using buffer manager
    pcmData, err := a.BufferManager.ReadSegmentFromCaptureBuffer(a.Note.Source, a.Note.BeginTime, 15)
    if err != nil {
        log.Printf("Error reading PCM data: %v", err)
        return err
    }
    
    // Save audio to database
    return saveAudio(pcmData)
}
```

### Handling Edge Cases

#### Source ID Standardization

Both versions of the source ID (with or without the "device:" prefix) will work with the new buffer manager, as it internally handles standardization:

```go
// Both of these will find the same buffer
bufferManager.HasCaptureBuffer("microphone1")
bufferManager.HasCaptureBuffer("device:microphone1")
```

#### Thread Safety

The new package properly handles thread safety, so you don't need to add your own locks:

```go
// This is now safe for concurrent access from multiple goroutines
go func() {
    bufferManager.WriteToCaptureBuffer("source1", data1)
}()
go func() {
    bufferManager.ReadSegmentFromCaptureBuffer("source1", time.Now(), 10)
}()
```

#### Error Handling

The new package provides more consistent error handling:

```go
// Example of improved error handling
segment, err := bufferManager.ReadSegmentFromCaptureBuffer(sourceID, startTime, duration)
if err != nil {
    if errors.Is(err, buffer.ErrBufferNotFound) {
        // Handle missing buffer case
        log.Printf("No buffer found for source %s", sourceID)
    } else if errors.Is(err, buffer.ErrSegmentNotAvailable) {
        // Handle case where segment is outside buffer timeframe
        log.Printf("Requested segment not available in buffer")
    } else {
        // Handle other errors
        log.Printf("Error reading segment: %v", err)
    }
    return err
}
```

## Thread Safety

All buffer implementations use appropriate mutex locks to ensure thread-safe operations:
- Analysis buffers use Read/Write mutexes for optimal concurrency
- Capture buffers use standard mutexes for sequential read/write operations
- Buffer manager implements separate locks for analysis and capture operations
- Lock granularity is optimized to minimize contention while ensuring data integrity

## Error Handling

The package follows Go error handling best practices:
- All operations return meaningful errors with proper context
- Errors are wrapped using fmt.Errorf with the %w verb for proper error chaining
- Retry logic implemented for critical operations like buffer writes
- Comprehensive error messages that include buffer state information
- Graceful degradation with appropriate fallbacks when possible

## Resource Management

- Buffer capacity calculations account for sample rate, channels, and bit depth
- Efficient memory usage through ring buffer implementation
- Aligned buffer sizes for optimal memory access patterns
- Proper handling of buffer overflow conditions with warning mechanisms
- Graceful degradation under memory pressure
- Explicit cleanup methods to prevent resource leaks

## API Reference

### Types

#### Core Interfaces

**`AudioBuffer`** (`interfaces.go`): Base interface for all buffer types
- Methods:
  - `Write(data []byte) (int, error)`: Writes audio data to the buffer
  - `Read(p []byte) (int, error)`: Reads audio data from the buffer
  - `Reset() error`: Resets the buffer to its initial state
  - `SampleRate() uint32`: Returns the buffer's sample rate in Hz
  - `Channels() uint32`: Returns the number of audio channels

**`AnalysisBufferInterface`** (`interfaces.go`): Interface for analysis-specific buffer operations
- Extends `AudioBuffer` interface
- Methods:
  - `ReadyForAnalysis() bool`: Checks if enough data is available for analysis

**`CaptureBufferInterface`** (`interfaces.go`): Interface for capture-specific buffer operations
- Extends `AudioBuffer` interface
- Methods:
  - `ReadSegment(startTime time.Time, durationSeconds int) ([]byte, error)`: Extracts a time-based audio segment

**`BufferManagerInterface`** (`interfaces.go`): Interface for buffer management operations
- Methods:
  - `AllocateAnalysisBuffer(capacity int, source string) error`: Allocates an analysis buffer for a source
  - `RemoveAnalysisBuffer(source string) error`: Removes an analysis buffer for a source
  - `AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error`: Allocates a capture buffer
  - `RemoveCaptureBuffer(source string) error`: Removes a capture buffer for a source
  - `WriteToAnalysisBuffer(stream string, data []byte) error`: Writes data to an analysis buffer
  - `ReadFromAnalysisBuffer(stream string, config *BufferConfig) ([]byte, error)`: Reads data from an analysis buffer
  - `WriteToCaptureBuffer(source string, data []byte) error`: Writes data to a capture buffer
  - `ReadSegmentFromCaptureBuffer(source string, startTime time.Time, duration int) ([]byte, error)`: Reads a segment from a capture buffer
  - `CleanupAllBuffers()`: Cleans up all buffers
  - `HasAnalysisBuffer(source string) bool`: Checks if an analysis buffer exists for a source
  - `HasCaptureBuffer(source string) bool`: Checks if a capture buffer exists for a source

**`RingBufferInterface`** (`interfaces.go`): Interface for ring buffer operations
- Methods:
  - `Write(p []byte) (n int, err error)`: Writes data to the ring buffer
  - `Read(p []byte) (n int, err error)`: Reads data from the ring buffer
  - `Length() int`: Returns the number of bytes in the buffer
  - `Capacity() int`: Returns the total capacity of the buffer
  - `Free() int`: Returns the number of free bytes in the buffer
  - `Reset()`: Resets the buffer

**`BufferFactoryInterface`** (`interfaces.go`): Interface for buffer factory operations
- Methods:
  - `CreateAnalysisBuffer(sampleRate, channels uint32, duration time.Duration) AnalysisBufferInterface`: Creates an analysis buffer
  - `CreateCaptureBuffer(sampleRate, channels uint32, duration time.Duration) CaptureBufferInterface`: Creates a capture buffer
  - `CreateBufferManager() BufferManagerInterface`: Creates a buffer manager

#### Implementation Classes

**`AnalysisBuffer`** (`analysis.go`): Implementation of `AnalysisBufferInterface`
- Fields:
  - `buffer`: The underlying ring buffer
  - `prevData`: Previous data for sliding window
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `threshold`: Analysis readiness threshold
  - `overlapFraction`: Overlap fraction for sliding window
  - `warningCounter`: Counter for buffer-full warnings
  - `mu`: Mutex for thread safety
  - `logger`: Logger interface
  - `timeProvider`: Time provider interface

**`CaptureBuffer`** (`capture.go`): Implementation of `CaptureBufferInterface`
- Fields:
  - `data`: The underlying byte array
  - `writeIndex`: Current write position
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `bytesPerSample`: Bytes per audio sample
  - `bufferSize`: Total buffer size in bytes
  - `bufferDuration`: Duration of the buffer
  - `startTime`: Buffer start time
  - `initialized`: Whether buffer is initialized
  - `mu`: Mutex for thread safety
  - `logger`: Logger interface
  - `timeProvider`: Time provider interface

**`BufferManager`** (`buffer_manager.go`): Implementation of `BufferManagerInterface`
- Fields:
  - `analysisBuffers`: Map of analysis buffers by source ID
  - `analysisMutex`: Mutex for analysis buffers
  - `captureBuffers`: Map of capture buffers by source ID
  - `captureMutex`: Mutex for capture buffers
  - `logger`: Logger interface
  - `timeProvider`: Time provider interface
  - `config`: Buffer configuration
  - `bufferFactory`: Buffer factory interface

**`BufferFactory`** (`buffer_factory.go`): Implementation of `BufferFactoryInterface`
- Fields:
  - `logger`: Logger interface
  - `timeProvider`: Time provider interface
  - `config`: Buffer configuration
  - `ringBufferFactory`: Factory function for ring buffers

**`BufferConfig`** (`buffer_config.go`): Configuration for buffer operations
- Fields:
  - `BirdNETOverlap`: Overlap fraction for BirdNET
  - `SampleRate`: Default sample rate
  - `BitDepth`: Default bit depth
  - `BufferSize`: Default buffer size
  - `DefaultAnalysisDuration`: Default duration for analysis buffers
  - `DefaultCaptureDuration`: Default duration for capture buffers

#### Dependencies

**`TimeProvider`** (`dependencies.go`): Interface for time-related operations
- Methods:
  - `Now() time.Time`: Returns the current time
  - `Sleep(duration time.Duration)`: Sleeps for the specified duration

**`RealTimeProvider`** (`dependencies.go`): Default implementation of `TimeProvider`
- Methods:
  - `Now() time.Time`: Returns the current system time
  - `Sleep(duration time.Duration)`: Sleeps using system time

**`Logger`** (`dependencies.go`): Interface for logging operations
- Methods:
  - `Debug(msg string, args ...interface{})`: Logs a debug message
  - `Info(msg string, args ...interface{})`: Logs an info message
  - `Warn(msg string, args ...interface{})`: Logs a warning message
  - `Error(msg string, args ...interface{})`: Logs an error message

**`StandardLogger`** (`dependencies.go`): Default implementation of `Logger`
- Methods:
  - `Debug(msg string, args ...interface{})`: Logs a debug message
  - `Info(msg string, args ...interface{})`: Logs an info message
  - `Warn(msg string, args ...interface{})`: Logs a warning message
  - `Error(msg string, args ...interface{})`: Logs an error message

### Functions

#### Analysis Buffer

**`NewAnalysisBuffer(sampleRate, channels uint32, duration time.Duration) *AnalysisBuffer`** (`analysis.go`): Creates a new analysis buffer with default dependencies
- Arguments:
  - `sampleRate`: Sample rate in Hz (e.g., 44100, 48000)
  - `channels`: Number of audio channels
  - `duration`: Duration of the buffer
- Returns:
  - A new analysis buffer

**`NewAnalysisBufferWithDeps(sampleRate, channels uint32, duration time.Duration, ringBufferFactory func(size int) RingBufferInterface, logger Logger, timeProvider TimeProvider) *AnalysisBuffer`** (`analysis.go`): Creates a new analysis buffer with custom dependencies
- Arguments:
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `duration`: Duration of the buffer
  - `ringBufferFactory`: Factory function for ring buffers
  - `logger`: Logger implementation
  - `timeProvider`: Time provider implementation
- Returns:
  - A new analysis buffer with custom dependencies

#### AnalysisBuffer Methods

**`Write(data []byte) (int, error)`** (`analysis.go`): Writes audio data to the buffer
- Arguments:
  - `data`: Audio data to write
- Returns:
  - Number of bytes written and any error

**`Read(p []byte) (int, error)`** (`analysis.go`): Reads audio data from the buffer with sliding window approach
- Arguments:
  - `p`: Buffer to read into
- Returns:
  - Number of bytes read and any error

**`ReadyForAnalysis() bool`** (`analysis.go`): Checks if buffer has enough data for analysis
- Returns:
  - Whether the buffer is ready for analysis

**`Reset() error`** (`analysis.go`): Resets the buffer to its initial state
- Returns:
  - Any error that occurred

**`SampleRate() uint32`** (`analysis.go`): Returns the buffer's sample rate
- Returns:
  - Sample rate in Hz

**`Channels() uint32`** (`analysis.go`): Returns the number of audio channels
- Returns:
  - Number of audio channels

**`SetOverlapFraction(fraction float64)`** (`analysis.go`): Sets the overlap fraction for sliding window
- Arguments:
  - `fraction`: Overlap fraction (0.0-1.0)

#### Capture Buffer

**`NewCaptureBuffer(sampleRate, channels uint32, duration time.Duration) *CaptureBuffer`** (`capture.go`): Creates a new capture buffer with default dependencies
- Arguments:
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `duration`: Duration of the buffer
- Returns:
  - A new capture buffer

**`NewCaptureBufferWithDeps(sampleRate, channels uint32, duration time.Duration, logger Logger, timeProvider TimeProvider) *CaptureBuffer`** (`capture.go`): Creates a new capture buffer with custom dependencies
- Arguments:
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `duration`: Duration of the buffer
  - `logger`: Logger implementation
  - `timeProvider`: Time provider implementation
- Returns:
  - A new capture buffer with custom dependencies

#### CaptureBuffer Methods

**`Write(data []byte) (bytesWritten int, err error)`** (`capture.go`): Adds PCM audio data to the buffer
- Arguments:
  - `data`: Audio data to write
- Returns:
  - Number of bytes written and any error

**`Read(p []byte) (bytesRead int, err error)`** (`capture.go`): Reads audio data from the buffer
- Arguments:
  - `p`: Buffer to read into
- Returns:
  - Number of bytes read and any error

**`ReadSegment(requestedStartTime time.Time, durationSeconds int) (segment []byte, err error)`** (`capture.go`): Extracts a segment of audio data based on start time and duration
- Arguments:
  - `requestedStartTime`: Segment start time
  - `durationSeconds`: Segment duration in seconds
- Returns:
  - Audio segment and any error

**`Reset() error`** (`capture.go`): Resets the buffer to its initial state
- Returns:
  - Any error that occurred

**`SampleRate() uint32`** (`capture.go`): Returns the buffer's sample rate
- Returns:
  - Sample rate in Hz

**`Channels() uint32`** (`capture.go`): Returns the number of audio channels
- Returns:
  - Number of audio channels

#### Buffer Manager

**`NewBufferManager() *BufferManager`** (`buffer_manager.go`): Creates a new buffer manager with default dependencies
- Returns:
  - A new buffer manager

**`NewBufferManagerWithDeps(logger Logger, timeProvider TimeProvider, config *BufferConfig, bufferFactory BufferFactoryInterface) *BufferManager`** (`buffer_manager.go`): Creates a new buffer manager with custom dependencies
- Arguments:
  - `logger`: Logger implementation
  - `timeProvider`: Time provider implementation
  - `config`: Buffer configuration
  - `bufferFactory`: Buffer factory interface
- Returns:
  - A new buffer manager with custom dependencies

#### BufferManager Methods

**`AllocateAnalysisBuffer(capacity int, source string) error`** (`buffer_manager.go`): Initializes a ring buffer for a single audio source
- Arguments:
  - `capacity`: Buffer capacity in bytes
  - `source`: Source identifier
- Returns:
  - Any error that occurred

**`RemoveAnalysisBuffer(source string) error`** (`buffer_manager.go`): Safely removes and cleans up a ring buffer for a single source
- Arguments:
  - `source`: Source identifier
- Returns:
  - Any error that occurred

**`AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error`** (`buffer_manager.go`): Initializes a capture buffer for a single source
- Arguments:
  - `durationSeconds`: Buffer duration in seconds
  - `sampleRate`: Sample rate in Hz
  - `bytesPerSample`: Bytes per audio sample
  - `source`: Source identifier
- Returns:
  - Any error that occurred

**`RemoveCaptureBuffer(source string) error`** (`buffer_manager.go`): Safely removes a capture buffer for a single source
- Arguments:
  - `source`: Source identifier
- Returns:
  - Any error that occurred

**`WriteToAnalysisBuffer(stream string, data []byte) error`** (`buffer_manager.go`): Writes audio data into the analysis buffer for a given stream
- Arguments:
  - `stream`: Stream identifier
  - `data`: Audio data to write
- Returns:
  - Any error that occurred

**`ReadFromAnalysisBuffer(stream string, optionalConfig *BufferConfig) ([]byte, error)`** (`buffer_manager.go`): Reads a sliding chunk of audio data from the buffer for a given stream
- Arguments:
  - `stream`: Stream identifier
  - `optionalConfig`: Optional buffer configuration
- Returns:
  - Audio data and any error

**`WriteToCaptureBuffer(source string, data []byte) error`** (`buffer_manager.go`): Writes audio data to the capture buffer for a given source
- Arguments:
  - `source`: Source identifier
  - `data`: Audio data to write
- Returns:
  - Any error that occurred

**`ReadSegmentFromCaptureBuffer(source string, requestedStartTime time.Time, duration int) ([]byte, error)`** (`buffer_manager.go`): Reads a time-based segment from the capture buffer
- Arguments:
  - `source`: Source identifier
  - `requestedStartTime`: Segment start time
  - `duration`: Segment duration in seconds
- Returns:
  - Audio segment and any error

**`CleanupAllBuffers()`** (`buffer_manager.go`): Cleans up all allocated buffers
- No arguments
- No return value

**`HasAnalysisBuffer(source string) bool`** (`buffer_manager.go`): Checks if an analysis buffer exists for a given source
- Arguments:
  - `source`: Source identifier
- Returns:
  - Whether the buffer exists

**`HasCaptureBuffer(source string) bool`** (`buffer_manager.go`): Checks if a capture buffer exists for a given source
- Arguments:
  - `source`: Source identifier
- Returns:
  - Whether the buffer exists

#### Buffer Factory

**`NewBufferFactory() *BufferFactory`** (`buffer_factory.go`): Creates a buffer factory with default dependencies
- Returns:
  - A new buffer factory

**`NewBufferFactoryWithDeps(logger Logger, timeProvider TimeProvider, config *BufferConfig, ringBufferFactory func(size int) RingBufferInterface) *BufferFactory`** (`buffer_factory.go`): Creates a buffer factory with custom dependencies
- Arguments:
  - `logger`: Logger implementation
  - `timeProvider`: Time provider implementation
  - `config`: Buffer configuration
  - `ringBufferFactory`: Factory function for ring buffers
- Returns:
  - A new buffer factory with custom dependencies

#### BufferFactory Methods

**`CreateAnalysisBuffer(sampleRate, channels uint32, duration time.Duration) AnalysisBufferInterface`** (`buffer_factory.go`): Creates a new analysis buffer
- Arguments:
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `duration`: Buffer duration
- Returns:
  - A new analysis buffer

**`CreateCaptureBuffer(sampleRate, channels uint32, duration time.Duration) CaptureBufferInterface`** (`buffer_factory.go`): Creates a new capture buffer
- Arguments:
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
  - `duration`: Buffer duration
- Returns:
  - A new capture buffer

**`CreateBufferManager() BufferManagerInterface`** (`buffer_factory.go`): Creates a new buffer manager
- Returns:
  - A new buffer manager

#### Buffer Configuration

**`NewDefaultBufferConfig() *BufferConfig`** (`buffer_config.go`): Creates a config with sensible defaults
- Returns:
  - A new buffer configuration with default values

## Cross-Platform Considerations

- No platform-specific code in the buffer implementation
- Platform-independent time management through TimeProvider abstraction
- Consistent byte order handling for audio data
- **Important**: Each buffer is designed to handle a single audio channel; for multi-channel audio (stereo), use separate buffers
- Optimized for cross-platform performance
- Comprehensive test suite ensuring consistent behavior across platforms

## Dependencies

- **github.com/smallnest/ringbuffer**: Efficient ring buffer implementation used by AnalysisBuffer
- **sync**: For mutex and read/write mutex implementations (`sync.Mutex`, `sync.RWMutex`)
- **time**: For time-related operations (`time.Now()`, `time.Duration`, etc.)
- **errors**: For error handling and wrapping (`errors.New`, `errors.Is`, etc.)
- **fmt**: For string formatting and error wrapping (`fmt.Errorf`)
- **log**: For optional debug logging

## Implementation Notes

### Analysis Buffer
- Optimized for the BirdNET sliding window algorithm
- **Important**: Designed for single audio channel per buffer; do not store multi-channel (e.g., stereo) audio in a single buffer
- Default overlap is 0.25 (25%) but configurable via SetOverlapFraction
- Implements retry logic for write operations during buffer congestion
- Warning system to detect and report buffer overflow conditions
- Threshold-based readiness detection for efficient polling

### Capture Buffer
- **Important**: Designed for single audio channel per buffer; do not store multi-channel (e.g., stereo) audio in a single buffer
- Circular buffer with timestamp tracking for precise audio segment retrieval
- Adjusts timestamps when buffer wraps around to maintain time accuracy
- Aligns buffer size to optimize memory access patterns (2KB boundaries)
- Timeout mechanism for segment requests that cannot be fulfilled
- Efficient memory management with single underlying byte slice

## Performance Considerations

The buffer package is optimized for audio processing efficiency:

- **Memory allocation**: Pre-allocated buffers minimize GC pressure during audio processing
  - Ring buffer uses a single contiguous memory block
  - Capture buffer aligns to 2KB boundaries for improved memory access patterns
  - Buffer sizes are calculated based on audio parameters to avoid over-allocation

- **Concurrent access**: Optimized for real-time audio processing
  - Read/Write mutex in analysis buffer allows concurrent reads
  - Lock granularity is designed to minimize contention
  - Retry logic handles temporary congestion situations

- **CPU efficiency**:
  - Minimized copying operations in buffer implementations
  - Efficient sliding window implementation with configurable overlap
  - Time-based segment extraction with minimal computation

- **Scalability**:
  - Buffer manager supports multiple concurrent audio sources
  - Independent buffer allocation per source prevents cross-interference
  - Separate mutex locks for analysis and capture operations

## Testing

The buffer package includes comprehensive tests for all key components:

- **Unit tests** for each buffer implementation
  - AnalysisBuffer tests verify sliding window behavior and thread safety
  - CaptureBuffer tests ensure accurate time-based segment retrieval
  - BufferManager tests confirm proper buffer lifecycle management
  - BufferFactory tests validate correct configuration of created components

- **Mocks and stubs** for improved testability
  - MockTimeProvider allows time manipulation for deterministic testing
  - MockLogger enables verification of logging behavior
  - MockRingBuffer provides controlled buffer behavior for edge case testing

- **Test coverage** prioritizes critical code paths
  - Error handling paths for buffer overflow conditions
  - Concurrent access patterns with race condition detection
  - Edge cases like buffer wrap-around and time synchronization
  - Configuration validation and parameter boundary testing

- **Integration tests** ensure components work together correctly
  - End-to-end audio pipeline tests from input to analysis
  - Performance benchmarks for key operations

Run tests with race detection enabled to verify thread safety:
```go
go test -race ./internal/audio/buffer/...
``` 