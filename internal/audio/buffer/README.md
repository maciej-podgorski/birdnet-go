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

- **AnalysisBufferInterface**: Interface for analysis-specific buffer operations
  - Extends AudioBuffer with analysis-specific functionality
  - Provides methods for checking analysis readiness

- **CaptureBufferInterface**: Interface for capture-specific buffer operations
  - Extends AudioBuffer with capture-specific functionality
  - Enables extraction of time-based audio segments

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
- Optimized for cross-platform performance
- Comprehensive test suite ensuring consistent behavior across platforms

## Dependencies

- **github.com/smallnest/ringbuffer**: Efficient ring buffer implementation used by AnalysisBuffer
- Standard Go libraries for core functionality (sync, time, errors)

## Implementation Notes

### Analysis Buffer
- Optimized for the BirdNET sliding window algorithm
- Default overlap is 0.25 (25%) but configurable via SetOverlapFraction
- Implements retry logic for write operations during buffer congestion
- Warning system to detect and report buffer overflow conditions
- Threshold-based readiness detection for efficient polling

### Capture Buffer
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