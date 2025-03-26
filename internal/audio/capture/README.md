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

## API Reference

### Types

#### Core Interfaces and Adapters

**`MalgoContextAdapter`** (`context.go`): Adapts malgo.AllocatedContext to the AudioContext interface
- Fields:
  - `context`: The underlying malgo.AllocatedContext
  - `mu`: Mutex for thread safety
- Methods:
  - `Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error)`: Lists audio devices
  - `InitDevice(config *malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error)`: Initializes a device
  - `Uninit() error`: Uninitializes the context
  - `GetRawContext() *malgo.AllocatedContext`: Returns the underlying context

**`MalgoDeviceAdapter`** (`device.go`): Adapts malgo.Device to the AudioDevice interface
- Fields:
  - `device`: The underlying malgo.Device
  - `mu`: Mutex for thread safety
- Methods:
  - `Start() error`: Starts the device
  - `Stop() error`: Stops the device
  - `Uninit() error`: Uninitializes the device
  - `GetRawDevice() *malgo.Device`: Returns the underlying device

**`ContextFactory`** (`capture_factory.go`): Factory for creating audio contexts
- Fields:
  - `debug`: Debug mode flag
- Methods:
  - `CreateAudioContext(logger func(string)) (audio.AudioContext, error)`: Creates a new audio context
  
**`DeviceManager`** (`device_manager.go`): Manages audio capture devices
- Fields:
  - `context`: Audio context for device operations
  - `bufferManager`: Buffer manager for audio data
  - `devices`: Map of device states
  - `callbackMap`: Map of device callbacks
  - `dataCallback`: Callback for audio data
  - `stopCallback`: Callback for device stops
  - `restartEnabled`: Whether automatic restart is enabled
  - `restartDelay`: Delay between restart attempts
  - `quitChan`: Channel for signaling shutdown
  - `mu`: Mutex for thread safety
  - `wg`: WaitGroup for goroutine synchronization

**`deviceState`** (`device_manager.go`): Tracks the state of a capture device
- Fields:
  - `info`: Device information
  - `device`: Audio device instance
  - `sourceID`: Unique source identifier
  - `isActive`: Whether the device is active
  - `restartAttempts`: Count of restart attempts

**`captureDeviceInfo`** (`device_manager.go`): Stores device identification information
- Fields:
  - `Name`: Device name
  - `ID`: Device ID
  - `Pointer`: Device pointer for malgo

### Functions

#### Context and Device Creation

**`NewMalgoContextAdapter(backends []malgo.Backend, config *malgo.ContextConfig, logger func(string)) (audio.AudioContext, error)`** (`context.go`): Creates a new context adapter
- Arguments:
  - `backends`: Audio backends to use
  - `config`: Context configuration
  - `logger`: Optional logging function
- Returns:
  - Audio context and any error

**`NewMalgoDeviceAdapter(device *malgo.Device) audio.AudioDevice`** (`device.go`): Creates a new device adapter
- Arguments:
  - `device`: Malgo device to adapt
- Returns:
  - Audio device implementation

**`NewContextFactory(debug bool) *ContextFactory`** (`capture_factory.go`): Creates a new context factory
- Arguments:
  - `debug`: Whether to enable debug mode
- Returns:
  - A new context factory

**`DefaultAudioContextFactory() audio.AudioContextFactory`** (`capture_factory.go`): Returns the default factory implementation
- Returns:
  - Default audio context factory

#### Device Management

**`NewDeviceManager(context audio.AudioContext, bufferManager audio.BufferManager) *DeviceManager`** (`device_manager.go`): Creates a new device manager
- Arguments:
  - `context`: Audio context for device operations
  - `bufferManager`: Buffer manager for audio data
- Returns:
  - A new device manager

#### Utility Functions

**`HexToASCII(hexStr string) (string, error)`** (`utils.go`): Converts a hexadecimal string to ASCII
- Arguments:
  - `hexStr`: Hexadecimal string
- Returns:
  - ASCII string and any error

**`TestDevice(ctx audio.AudioContext, deviceID string, sampleRate, channels uint32) bool`** (`utils.go`): Tests if a device can be used with the given parameters
- Arguments:
  - `ctx`: Audio context
  - `deviceID`: Device identifier
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
- Returns:
  - Whether the device can be used

**`ValidateAudioDevice(deviceID string, ctx audio.AudioContext, sampleRate, channels uint32) error`** (`utils.go`): Validates a device configuration
- Arguments:
  - `deviceID`: Device identifier
  - `ctx`: Audio context
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
- Returns:
  - Error if validation fails

**`ConvertMalgoDeviceInfo(info *malgo.DeviceInfo) (audio.DeviceInfo, error)`** (`utils.go`): Converts malgo device info to audio device info
- Arguments:
  - `info`: Malgo device info
- Returns:
  - Audio device info and any error

**`isHardwareDevice(deviceID string) bool`** (`utils.go`): Checks if a device ID represents hardware
- Arguments:
  - `deviceID`: Device identifier
- Returns:
  - Whether the device is hardware

**`matchesDeviceSettings(decodedID, deviceName, configuredID string) bool`** (`utils.go`): Checks if a device matches configuration
- Arguments:
  - `decodedID`: Decoded device ID
  - `deviceName`: Device name
  - `configuredID`: Configured ID
- Returns:
  - Whether the device matches

### Methods

#### ContextFactory Methods

**`CreateAudioContext(logger func(string)) (audio.AudioContext, error)`** (`capture_factory.go`): Creates a new audio context
- Arguments:
  - `logger`: Optional logging function
- Returns:
  - Audio context and any error

#### MalgoContextAdapter Methods

**`Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error)`** (`context.go`): Lists audio devices
- Arguments:
  - `deviceType`: Type of devices to list
- Returns:
  - List of device info and any error

**`InitDevice(config *malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error)`** (`context.go`): Initializes a device
- Arguments:
  - `config`: Device configuration
  - `callbacks`: Device callbacks
- Returns:
  - Malgo device and any error

**`Uninit() error`** (`context.go`): Uninitializes the context
- Returns:
  - Any error that occurred

**`GetRawContext() *malgo.AllocatedContext`** (`context.go`): Returns the underlying context
- Returns:
  - The underlying malgo context

#### MalgoDeviceAdapter Methods

**`Start() error`** (`device.go`): Starts the device
- Returns:
  - Any error that occurred

**`Stop() error`** (`device.go`): Stops the device
- Returns:
  - Any error that occurred

**`Uninit() error`** (`device.go`): Uninitializes the device
- Returns:
  - Any error that occurred

**`GetRawDevice() *malgo.Device`** (`device.go`): Returns the underlying device
- Returns:
  - The underlying malgo device

#### DeviceManager Methods

**`SetDataCallback(callback func(sourceID string, data []byte, frameCount uint32))`** (`device_manager.go`): Sets the data callback
- Arguments:
  - `callback`: Function to call when audio data arrives

**`SetStopCallback(callback func(sourceID string))`** (`device_manager.go`): Sets the stop callback
- Arguments:
  - `callback`: Function to call when a device stops

**`SetRestartOptions(enabled bool, delay time.Duration)`** (`device_manager.go`): Configures automatic restart behavior
- Arguments:
  - `enabled`: Whether to enable automatic restart
  - `delay`: Delay between restart attempts

**`StartCapture(deviceID string, sampleRate, channels uint32) error`** (`device_manager.go`): Starts audio capture from a device
- Arguments:
  - `deviceID`: Device identifier
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
- Returns:
  - Any error that occurred

**`StopCapture(deviceID string) error`** (`device_manager.go`): Stops audio capture from a device
- Arguments:
  - `deviceID`: Device identifier
- Returns:
  - Any error that occurred

**`ListDevices() ([]audio.DeviceInfo, error)`** (`device_manager.go`): Lists all available capture devices
- Returns:
  - List of device info and any error

**`GetDevice(deviceID string) (audio.DeviceInfo, error)`** (`device_manager.go`): Gets information about a specific device
- Arguments:
  - `deviceID`: Device identifier
- Returns:
  - Device info and any error

**`Close() error`** (`device_manager.go`): Stops all devices and closes the manager
- Returns:
  - Any error that occurred

**`GetPlatformSpecificDevices() ([]audio.DeviceInfo, error)`** (`device_manager.go`): Gets platform-specific devices
- Returns:
  - List of device info and any error

**`createAndStartDevice(deviceID string, deviceInfo captureDeviceInfo, sourceID string, sampleRate, channels uint32) (audio.AudioDevice, error)`** (`device_manager.go`): Creates and starts a device
- Arguments:
  - `deviceID`: Device identifier
  - `deviceInfo`: Device information
  - `sourceID`: Source identifier
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels
- Returns:
  - Audio device and any error

**`attemptDeviceRestart(deviceID string, deviceInfo captureDeviceInfo, sampleRate, channels uint32)`** (`device_manager.go`): Attempts to restart a failed device
- Arguments:
  - `deviceID`: Device identifier
  - `deviceInfo`: Device information
  - `sampleRate`: Sample rate in Hz
  - `channels`: Number of audio channels

**`getDeviceInfo(deviceID string) (captureDeviceInfo, error)`** (`device_manager.go`): Gets internal device info
- Arguments:
  - `deviceID`: Device identifier
- Returns:
  - Capture device info and any error

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

- github.com/tphakala/malgo: Cross-platform audio library (fork of gen2brain/malgo)
- Internal audio interfaces defined in internal/audio
- Standard library: sync, time, log, fmt, strings, etc.
