package capture

import (
	"testing"

	"github.com/tphakala/malgo"
)

func TestMalgoContextAdapter(t *testing.T) {
	// Use null backend for testing to avoid hardware dependencies
	backends := []malgo.Backend{malgo.BackendNull}

	// Create a new adapter
	adapter, err := NewMalgoContextAdapter(backends, &malgo.ContextConfig{}, nil)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// Clean up after test
	defer adapter.Uninit()

	// Test devices method
	devices, err := adapter.Devices(malgo.Capture)
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}

	// With null backend, we expect at least one device (the null device)
	if len(devices) < 1 {
		t.Errorf("Expected at least one device, got %d", len(devices))
	}

	// Test getting raw context
	if _, ok := adapter.(*MalgoContextAdapter); !ok {
		t.Errorf("Adapter is not of expected type")
	}

	malgoAdapter := adapter.(*MalgoContextAdapter)
	rawContext := malgoAdapter.GetRawContext()
	if rawContext == nil {
		t.Errorf("GetRawContext returned nil")
	}
}

func TestMalgoDeviceAdapter(t *testing.T) {
	// Since we can't easily create a real device for testing,
	// we'll just test the constructor and type checking

	// Test NewMalgoDeviceAdapter with nil (invalid, but for type checking only)
	adapter := NewMalgoDeviceAdapter(nil)

	// Verify type
	if _, ok := adapter.(*MalgoDeviceAdapter); !ok {
		t.Errorf("Adapter is not of expected type")
	}
}

func TestContextFactory(t *testing.T) {
	// Create a new factory
	factory := NewContextFactory(true)

	// Test CreateAudioContext method
	// We'll use a custom logger to verify it's called
	loggerCalled := false
	logger := func(message string) {
		loggerCalled = true
	}

	// Create the context
	context, err := factory.CreateAudioContext(logger)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Verify the logger was called
	if !loggerCalled {
		t.Errorf("Logger was not called during context creation")
	}

	// Clean up
	defer context.Uninit()

	// Test default factory
	defaultFactory := DefaultAudioContextFactory()
	if defaultFactory == nil {
		t.Errorf("DefaultAudioContextFactory returned nil")
	}
}
