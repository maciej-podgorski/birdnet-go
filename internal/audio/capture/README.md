# Audio Capture Package

## Overview

The `capture` package provides cross-platform audio device management and capture functionality for the BirdNet-Go application. It abstracts hardware interactions through the miniaudio/malgo library to provide a consistent interface across different operating systems (Linux, macOS, and Windows).

This package is part of the larger [audio system](../README.md) which provides comprehensive audio processing capabilities for the BirdNet-Go application.

## Key Components

### Adapters
- **MalgoContextAdapter**: Implements the `audio.AudioContext` interface by adapting the malgo.AllocatedContext
- **MalgoDeviceAdapter**: Implements the `audio.AudioDevice` interface by adapting the malgo.Device

### Managers
- **DeviceManager**: Manages the lifecycle of audio capture devices, including:
  - Device enumeration and selection
  - Starting and stopping capture
  - Buffer management for analysis and recording
  - Thread-safe operations with mutex locks
  - Automatic device restart on failure (esp. for ALSA "poll() failed" errors)

### Factory
- **ContextFactory**: Creates platform-appropriate audio contexts
  - Selects the correct backend based on the OS platform
  - Provides factory method pattern for context creation

### Utilities
- Helper functions for device testing and validation
- Cross-platform device ID management
- Audio hardware compatibility checks
- Graceful error recovery and device restart mechanisms

## Thread Safety

All components implement proper mutex locking to ensure thread-safe operations in concurrent environments.

## Usage Examples

### Creating an Audio Context

```go
// Create a context factory 
factory := capture.NewContextFactory(false)

// Create an audio context with optional logger
context, err := factory.CreateAudioContext(func(msg string) {
    log.Printf("Audio: %s", msg)
})
if err != nil {
    log.Fatalf("Failed to create audio context: %v", err)
}
defer context.Uninit()
```

### Enumerating Audio Devices

```go
// List available capture devices
manager := capture.NewDeviceManager(context, bufferManager)
devices, err := manager.ListDevices()
if err != nil {
    log.Fatalf("Failed to list devices: %v", err)
}

// Print device information
for _, device := range devices {
    fmt.Printf("Device ID: %s, Name: %s\n", device.ID, device.Name)
}
```

### Starting Audio Capture with Callbacks

```go
// Create a device manager
manager := capture.NewDeviceManager(context, bufferManager)

// Set the data callback for receiving audio data
manager.SetDataCallback(func(sourceID string, data []byte, frameCount uint32) {
    // Process audio data
    // For example, send to analyzer or recorder
})

// Set the stop callback for handling device disconnections
manager.SetStopCallback(func(sourceID string) {
    log.Printf("Device %s was disconnected or stopped", sourceID)
    // Handle reconnection logic or notify user
})

// Start capture from a device with specified sample rate and channels
err = manager.StartCapture("deviceID", 48000, 1)
if err != nil {
    log.Fatalf("Failed to start capture: %v", err)
}
```

### Configuring Automatic Device Restart

```go
// Create a device manager
manager := capture.NewDeviceManager(context, bufferManager)

// Configure automatic restart behavior
// Arguments: enabled, delay between attempts
manager.SetRestartOptions(true, 1*time.Second)

// Start capture as normal - device will automatically restart if it fails
// with no limit on retry attempts
err = manager.StartCapture("deviceID", 48000, 1)
if err != nil {
    log.Fatalf("Failed to start capture: %v", err)
}
```

### Testing Device Compatibility

```go
// Test if a device can be used with specified parameters
isCompatible := capture.TestDevice(context, "deviceID", 44100, 1)
if !isCompatible {
    log.Printf("Device is not compatible with the specified parameters")
}
```

### Complete Cleanup

```go
// Stop all captures and clean up resources
err = manager.Close()
if err != nil {
    log.Printf("Error during cleanup: %v", err)
}

// Uninitialize the audio context
context.Uninit()
```

## Cross-Platform Considerations

- Uses proper platform detection via runtime.GOOS
- Handles device naming differences between operating systems
- Implements platform-specific optimizations where needed
- Supports ALSA on Linux with proper mmap configuration

## Resource Management

- Proper cleanup of devices and contexts with defer statements
- Error checking for all audio operations
- Appropriate buffer allocation and deallocation
- Prevention of memory leaks through systematic resource tracking

## Error Handling

- Detailed error context with proper error wrapping
- Early returns for error conditions
- Systematic cleanup on error paths
- Thread-safe error handling with mutex protection

## Testing

The package includes comprehensive tests for all components:
- Unit tests with mock implementations
- Integration tests with null audio backends
- Platform-specific test cases

## Dependencies

- github.com/gen2brain/malgo: Cross-platform audio library
- Internal audio interfaces defined in internal/audio
