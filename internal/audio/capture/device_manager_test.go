package capture

import (
	"runtime"
	"testing"
	"time"

	"github.com/tphakala/malgo"
)

// MockAudioContext implements the audio.AudioContext interface for testing
type MockAudioContext struct {
	devices []malgo.DeviceInfo
}

func (m *MockAudioContext) Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error) {
	return m.devices, nil
}

func (m *MockAudioContext) InitDevice(config *malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error) {
	// Return nil, which is invalid but is only for testing the flow
	return nil, nil
}

func (m *MockAudioContext) Uninit() error {
	return nil
}

// MockBufferManager implements the audio.BufferManager interface for testing
type MockBufferManager struct {
	buffers map[string]bool
}

func NewMockBufferManager() *MockBufferManager {
	return &MockBufferManager{
		buffers: make(map[string]bool),
	}
}

func (m *MockBufferManager) AllocateAnalysisBuffer(capacity int, source string) error {
	m.buffers[source+"_analysis"] = true
	return nil
}

func (m *MockBufferManager) AllocateCaptureBuffer(durationSeconds, sampleRate, bytesPerSample int, source string) error {
	m.buffers[source+"_capture"] = true
	return nil
}

func (m *MockBufferManager) RemoveAnalysisBuffer(source string) error {
	delete(m.buffers, source+"_analysis")
	return nil
}

func (m *MockBufferManager) RemoveCaptureBuffer(source string) error {
	delete(m.buffers, source+"_capture")
	return nil
}

func (m *MockBufferManager) HasAnalysisBuffer(source string) bool {
	_, exists := m.buffers[source+"_analysis"]
	return exists
}

func (m *MockBufferManager) HasCaptureBuffer(source string) bool {
	_, exists := m.buffers[source+"_capture"]
	return exists
}

func (m *MockBufferManager) WriteToAnalysisBuffer(stream string, data []byte) error {
	return nil
}

func (m *MockBufferManager) WriteToCaptureBuffer(source string, data []byte) error {
	return nil
}

func (m *MockBufferManager) ReadFromAnalysisBuffer(stream string) ([]byte, error) {
	return nil, nil
}

func (m *MockBufferManager) ReadSegmentFromCaptureBuffer(source string, requestedStartTime time.Time, duration int) ([]byte, error) {
	return nil, nil
}

func (m *MockBufferManager) CleanupAllBuffers() {
	m.buffers = make(map[string]bool)
}

func TestDeviceManager(t *testing.T) {
	// Create mock components
	mockContext := &MockAudioContext{
		devices: []malgo.DeviceInfo{
			// We can't easily create real DeviceInfo objects for testing
		},
	}

	mockBufferManager := NewMockBufferManager()

	// Create the device manager
	manager := NewDeviceManager(mockContext, mockBufferManager)

	// Test SetDataCallback
	callbackCalled := false
	manager.SetDataCallback(func(sourceID string, data []byte, frameCount uint32) {
		callbackCalled = true
	})

	// Since we can't fully test StartCapture without real devices,
	// we'll just test that methods don't panic

	// Test listing devices
	_, err := manager.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}

	// Test callback
	if !callbackCalled {
		t.Errorf("Callback was not called")
	}

	// Test Close
	err = manager.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Test GetPlatformSpecificDevices
	_, err = manager.GetPlatformSpecificDevices()
	if err != nil {
		t.Fatalf("GetPlatformSpecificDevices failed: %v", err)
	}
}

// Test utility functions
func TestUtilFunctions(t *testing.T) {
	// Test hexToASCII
	result, err := HexToASCII("4142")
	if err != nil {
		t.Fatalf("hexToASCII failed: %v", err)
	}
	if result != "AB" {
		t.Errorf("Expected 'AB', got '%s'", result)
	}

	// Test isHardwareDevice
	isHardware := isHardwareDevice(":0,0")
	if !isHardware && runtime.GOOS == "linux" {
		t.Errorf("Expected true for ':0,0' on Linux, got false")
	}

	// Test matchesDeviceSettings
	matches := matchesDeviceSettings("dev1", "Device 1", "dev")
	if !matches {
		t.Errorf("Expected true for matching device, got false")
	}
}
