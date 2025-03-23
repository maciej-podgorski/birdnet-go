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