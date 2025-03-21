package capture

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/malgo"
)

// DeviceCallback is a function type for device data callbacks.
type DeviceCallback func(data []byte, frameCount uint32)

// DeviceManager manages audio capture devices.
type DeviceManager struct {
	context       audio.AudioContext
	bufferManager audio.BufferManager

	devices     map[string]*deviceState
	callbackMap map[string]DeviceCallback

	dataCallback func(sourceID string, data []byte, frameCount uint32)
	stopCallback func(sourceID string)

	// Restart related fields
	restartEnabled bool
	restartDelay   time.Duration
	quitChan       chan struct{}

	mu sync.RWMutex
	wg sync.WaitGroup
}

// deviceState tracks the state of a capture device.
type deviceState struct {
	info            audio.DeviceInfo
	device          audio.AudioDevice
	sourceID        string
	isActive        bool
	restartAttempts int
}

// NewDeviceManager creates a new device manager.
func NewDeviceManager(context audio.AudioContext, bufferManager audio.BufferManager) *DeviceManager {
	return &DeviceManager{
		context:        context,
		bufferManager:  bufferManager,
		devices:        make(map[string]*deviceState),
		callbackMap:    make(map[string]DeviceCallback),
		restartEnabled: true,
		restartDelay:   500 * time.Millisecond,
		quitChan:       make(chan struct{}),
	}
}

// SetDataCallback sets the data callback function.
func (m *DeviceManager) SetDataCallback(callback func(sourceID string, data []byte, frameCount uint32)) {
	m.dataCallback = callback
}

// SetStopCallback sets the callback function that is called when a device stops.
func (m *DeviceManager) SetStopCallback(callback func(sourceID string)) {
	m.stopCallback = callback
}

// SetRestartOptions configures the automatic restart behavior for devices.
func (m *DeviceManager) SetRestartOptions(enabled bool, delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.restartEnabled = enabled
	m.restartDelay = delay
}

// createAndStartDevice creates a device with the given configuration and starts it
func (m *DeviceManager) createAndStartDevice(deviceID string, deviceInfo captureDeviceInfo, sourceID string, sampleRate, channels uint32) (audio.AudioDevice, error) {
	// Create device configuration
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = channels
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Alsa.NoMMap = 1
	deviceConfig.Periods = 3

	// Set the device ID based on the pointer type
	var deviceIDPtr unsafe.Pointer
	switch p := deviceInfo.Pointer.(type) {
	case unsafe.Pointer:
		deviceIDPtr = p
	case uintptr:
		// This unsafe.Pointer conversion is required for miniaudio C API compatibility.
		// The deviceInfo.Pointer comes from the malgo library and represents a hardware
		// resource that is managed by the underlying C library.
		deviceIDPtr = unsafe.Pointer(p)
	default:
		return nil, fmt.Errorf("invalid device pointer type: %T", deviceInfo.Pointer)
	}
	deviceConfig.Capture.DeviceID = deviceIDPtr

	// Create callbacks
	dataCallback := func(outputBuffer, inputBuffer []byte, frameCount uint32) {
		// Only log frame details at a debug level if needed
		// log.Printf("Received audio data: %d bytes, %d frames from %s", len(inputBuffer), frameCount, deviceInfo.Name)
		if m.dataCallback != nil {
			m.dataCallback(sourceID, inputBuffer, frameCount)
		}
	}

	stopCallback := func() {
		log.Printf("⚠️ Device stopped: %s (%s)", deviceInfo.Name, deviceID)

		// Try to restart the device if enabled
		if m.restartEnabled {
			go m.attemptDeviceRestart(deviceID, deviceInfo, sampleRate, channels)
		} else {
			// If restart is not enabled, just clean up normally
			go func() {
				m.mu.Lock()
				defer m.mu.Unlock()

				// Update device state if it still exists
				if state, exists := m.devices[deviceID]; exists {
					state.isActive = false
					state.device = nil
				}

				// Call the registered stop callback
				if m.stopCallback != nil {
					m.stopCallback(sourceID)
				}

				log.Printf("🛑 Device callback cleanup complete for: %s", deviceInfo.Name)
			}()
		}
	}

	callbacks := malgo.DeviceCallbacks{
		Data: dataCallback,
		Stop: stopCallback,
	}

	// Initialize the device
	malgoDevice, err := m.context.InitDevice(&deviceConfig, callbacks)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize device: %w", err)
	}

	// Create the device adapter
	device := NewMalgoDeviceAdapter(malgoDevice)

	// Start the device
	if err := device.Start(); err != nil {
		if uninitErr := device.Uninit(); uninitErr != nil {
			log.Printf("❌ Error uninitializing device: %v", uninitErr)
		}
		return nil, fmt.Errorf("failed to start device: %w", err)
	}

	return device, nil
}

// StartCapture starts audio capture from a device.
func (m *DeviceManager) StartCapture(deviceID string, sampleRate, channels uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if device is already active
	if state, exists := m.devices[deviceID]; exists && state.isActive {
		return fmt.Errorf("device is already active: %s", deviceID)
	}

	// Get device info
	deviceInfo, err := m.getDeviceInfo(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Generate a source ID for this device
	sourceID := fmt.Sprintf("device:%s", deviceID)

	// Store the callback in the map (for later reference)
	m.callbackMap[deviceID] = func(data []byte, frameCount uint32) {
		if m.dataCallback != nil {
			m.dataCallback(sourceID, data, frameCount)
		}
	}

	// Initialize buffers
	var analysisBufferOK, captureBufferOK bool

	// Initialize analysis buffer if it doesn't exist
	if m.bufferManager != nil && !m.bufferManager.HasAnalysisBuffer(sourceID) {
		if err := m.bufferManager.AllocateAnalysisBuffer(
			int(sampleRate*channels*2*3), // 3 seconds buffer
			sourceID,
		); err != nil {
			// Clean up on failure
			delete(m.callbackMap, deviceID)
			return fmt.Errorf("failed to create analysis buffer: %w", err)
		}
		analysisBufferOK = true
	} else {
		analysisBufferOK = true
	}

	// Initialize capture buffer if it doesn't exist
	if m.bufferManager != nil && !m.bufferManager.HasCaptureBuffer(sourceID) {
		if err := m.bufferManager.AllocateCaptureBuffer(
			60, // 60 seconds buffer
			int(sampleRate),
			2, // 2 bytes per sample (16-bit)
			sourceID,
		); err != nil {
			// Clean up on failure
			delete(m.callbackMap, deviceID)
			if analysisBufferOK {
				if rmErr := m.bufferManager.RemoveAnalysisBuffer(sourceID); rmErr != nil {
					log.Printf("❌ Error removing analysis buffer: %v", rmErr)
				}
			}
			return fmt.Errorf("failed to create capture buffer: %w", err)
		}
		captureBufferOK = true
	} else {
		captureBufferOK = true
	}

	// Create and start the device
	device, err := m.createAndStartDevice(deviceID, deviceInfo, sourceID, sampleRate, channels)
	if err != nil {
		// Clean up on failure
		delete(m.callbackMap, deviceID)
		if analysisBufferOK {
			if rmErr := m.bufferManager.RemoveAnalysisBuffer(sourceID); rmErr != nil {
				log.Printf("❌ Error removing analysis buffer: %v", rmErr)
			}
		}
		if captureBufferOK {
			if rmErr := m.bufferManager.RemoveCaptureBuffer(sourceID); rmErr != nil {
				log.Printf("❌ Error removing capture buffer: %v", rmErr)
			}
		}
		return fmt.Errorf("failed to create and start device: %w", err)
	}

	// Store the device state
	m.devices[deviceID] = &deviceState{
		info: audio.DeviceInfo{
			ID:       deviceID,
			Name:     deviceInfo.Name,
			IsInput:  true,
			Channels: channels,
		},
		device:          device,
		sourceID:        sourceID,
		isActive:        true,
		restartAttempts: 0,
	}

	log.Printf("🎤 Started listening on source: %s (%s)", deviceInfo.Name, deviceID)

	return nil
}

// attemptDeviceRestart attempts to restart a device that has stopped
func (m *DeviceManager) attemptDeviceRestart(deviceID string, deviceInfo captureDeviceInfo, sampleRate, channels uint32) {
	m.mu.Lock()

	// Check if the device still exists
	state, exists := m.devices[deviceID]
	if !exists {
		log.Printf("Device %s no longer exists, not attempting restart", deviceID)
		m.mu.Unlock()
		return
	}

	// Track restart attempts (for logging purposes only)
	state.restartAttempts++
	attemptCount := state.restartAttempts
	sourceID := state.sourceID

	m.mu.Unlock()

	// Wait before attempting restart
	log.Printf("🔄 Waiting %v before restart attempt #%d for device: %s",
		m.restartDelay, attemptCount, deviceInfo.Name)

	select {
	case <-m.quitChan:
		// Manager is shutting down, don't restart
		log.Printf("🛑 Device manager shutting down, cancelling restart of %s", deviceInfo.Name)
		return
	case <-time.After(m.restartDelay):
		// Continue with restart
	}

	log.Printf("🔄 Attempting restart #%d for device: %s", attemptCount, deviceInfo.Name)

	// Lock again to update the device
	m.mu.Lock()

	// Check if the device still exists
	state, exists = m.devices[deviceID]
	if !exists {
		log.Printf("Device %s no longer exists, aborting restart", deviceID)
		m.mu.Unlock()
		return
	}

	// Create and start the device
	device, err := m.createAndStartDevice(deviceID, deviceInfo, sourceID, sampleRate, channels)
	if err != nil {
		log.Printf("❌ Failed to restart device: %v", err)
		m.mu.Unlock()

		// Try again later
		go func() {
			select {
			case <-m.quitChan:
				return
			case <-time.After(m.restartDelay):
				m.attemptDeviceRestart(deviceID, deviceInfo, sampleRate, channels)
			}
		}()
		return
	}

	// Update device state
	state.device = device
	state.isActive = true

	m.mu.Unlock()

	log.Printf("✅ Successfully restarted device: %s", deviceInfo.Name)
}

// StopCapture stops audio capture from a device.
func (m *DeviceManager) StopCapture(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.devices[deviceID]
	if !exists || !state.isActive {
		return fmt.Errorf("device is not active: %s", deviceID)
	}

	// Get the source ID before we potentially remove it
	sourceID := state.sourceID

	// Stop the device
	if err := state.device.Stop(); err != nil {
		return fmt.Errorf("failed to stop device: %w", err)
	}

	// Uninitialize the device
	if err := state.device.Uninit(); err != nil {
		log.Printf("❌ Error uninitializing device %s: %v", state.info.Name, err)
	}

	// Remove the callback
	delete(m.callbackMap, deviceID)

	// Update the device state
	state.isActive = false
	state.device = nil

	log.Printf("🛑 Stopped listening on source: %s (%s)", state.info.Name, deviceID)

	// Call the stop callback
	if m.stopCallback != nil {
		m.stopCallback(sourceID)
	}

	return nil
}

// ListDevices returns a list of available audio devices.
func (m *DeviceManager) ListDevices() ([]audio.DeviceInfo, error) {
	// Get Malgo device infos
	malgoInfos, err := m.context.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("failed to get capture devices: %w", err)
	}

	// Convert Malgo device infos to our DeviceInfo type
	devices := make([]audio.DeviceInfo, 0, len(malgoInfos))
	for i := range malgoInfos {
		// Skip the discard/null device
		if malgoInfos[i].Name() == "Discard all samples" {
			continue
		}

		// Decode the device ID from hexadecimal to ASCII
		decodedID, err := HexToASCII(malgoInfos[i].ID.String())
		if err != nil {
			continue
		}

		devices = append(devices, audio.DeviceInfo{
			ID:       decodedID,
			Name:     malgoInfos[i].Name(),
			IsInput:  true,
			Channels: 2, // Default to stereo as fallback
		})
	}

	return devices, nil
}

// getDeviceInfo returns device info for a specific device ID.
func (m *DeviceManager) getDeviceInfo(deviceID string) (captureDeviceInfo, error) {
	// Get Malgo device infos
	malgoInfos, err := m.context.Devices(malgo.Capture)
	if err != nil {
		return captureDeviceInfo{}, fmt.Errorf("failed to get capture devices: %w", err)
	}

	// Find the matching device
	for i := range malgoInfos {
		decodedID, err := HexToASCII(malgoInfos[i].ID.String())
		if err != nil {
			continue
		}

		if decodedID == deviceID ||
			malgoInfos[i].Name() == deviceID ||
			strings.Contains(malgoInfos[i].Name(), deviceID) ||
			strings.Contains(decodedID, deviceID) {

			// Store the pointer directly without type assertion
			return captureDeviceInfo{
				Name:    malgoInfos[i].Name(),
				ID:      decodedID,
				Pointer: malgoInfos[i].ID.Pointer(),
			}, nil
		}
	}

	return captureDeviceInfo{}, fmt.Errorf("device not found: %s", deviceID)
}

// GetDevice returns information about a device.
func (m *DeviceManager) GetDevice(deviceID string) (audio.DeviceInfo, error) {
	// First check our active devices
	m.mu.RLock()
	if state, exists := m.devices[deviceID]; exists {
		info := state.info
		m.mu.RUnlock()
		return info, nil
	}
	m.mu.RUnlock()

	// If not found in active devices, check all available devices
	devices, err := m.ListDevices()
	if err != nil {
		return audio.DeviceInfo{}, fmt.Errorf("failed to get devices: %w", err)
	}

	for _, device := range devices {
		// More flexible matching for device ID
		if device.ID == deviceID ||
			device.Name == deviceID ||
			strings.Contains(device.Name, deviceID) ||
			strings.Contains(device.ID, deviceID) {
			return device, nil
		}
	}

	return audio.DeviceInfo{}, fmt.Errorf("device not found: %s", deviceID)
}

// Close closes the device manager and stops all devices.
func (m *DeviceManager) Close() error {
	// Signal that we're shutting down to prevent restarts
	close(m.quitChan)

	m.mu.Lock()

	// Get a list of all active devices
	activeDevices := make([]string, 0, len(m.devices))
	for deviceID, state := range m.devices {
		if state.isActive {
			activeDevices = append(activeDevices, deviceID)
		}
	}

	m.mu.Unlock()

	// Stop all active devices
	for _, deviceID := range activeDevices {
		if err := m.StopCapture(deviceID); err != nil {
			log.Printf("❌ Error stopping device %s: %v", deviceID, err)
		}
	}

	return nil
}

// GetPlatformSpecificDevices returns a list of devices relevant to the current platform.
func (m *DeviceManager) GetPlatformSpecificDevices() ([]audio.DeviceInfo, error) {
	devices, err := m.ListDevices()
	if err != nil {
		return nil, err
	}

	var filteredDevices []audio.DeviceInfo

	switch runtime.GOOS {
	case "linux":
		// On Linux, we want to filter out non-hardware devices
		for _, device := range devices {
			if isHardwareDevice(device.ID) {
				filteredDevices = append(filteredDevices, device)
			}
		}
	case "windows", "darwin":
		// On Windows and macOS, we keep all devices
		filteredDevices = devices
	}

	return filteredDevices, nil
}

// captureDeviceInfo holds detailed device information for capture initialization.
type captureDeviceInfo struct {
	Name    string
	ID      string
	Pointer interface{}
}
