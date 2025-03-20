package analysis

import (
	"fmt"
	"sync"

	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
)

var bn *birdnet.BirdNET           // BirdNET interpreter for legacy support
var instancesInitialized sync.Map // Track which model IDs have been initialized

// initializeBirdNET initializes the BirdNET interpreter and included species list if not already initialized.
func initializeBirdNET(settings *conf.Settings) error {
	// Initialize the BirdNET interpreter only if not already initialized
	if bn == nil {
		var err error
		bn, err = birdnet.NewBirdNET(settings)
		if err != nil {
			return fmt.Errorf("failed to initialize BirdNET: %w", err)
		}

		// Set the global instance
		birdnet.SetGlobalInstance(bn)

		// Also register this as the default instance
		birdnet.RegisterInstance("default", bn)

		// Initialize included species list
		if err := birdnet.BuildRangeFilter(bn); err != nil {
			return fmt.Errorf("failed to initialize BirdNET: %w", err)
		}
	}
	return nil
}

// InitializeBirdNETWithID initializes a BirdNET instance with a specific ID and model configuration
func InitializeBirdNETWithID(id string, settings *conf.Settings, modelPath, labelPath string) (*birdnet.BirdNET, error) {
	// Check if this instance is already initialized
	if _, initialized := instancesInitialized.Load(id); initialized {
		// Return the existing instance
		return birdnet.GetInstance(id), nil
	}

	// Create a copy of settings to avoid modifying the original
	instanceSettings := *settings

	// Override the model and label paths if provided
	if modelPath != "" {
		instanceSettings.BirdNET.ModelPath = modelPath
	}
	if labelPath != "" {
		instanceSettings.BirdNET.LabelPath = labelPath
	}

	// Create a new BirdNET instance
	instance, err := birdnet.NewBirdNET(&instanceSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize BirdNET instance %s: %w", id, err)
	}

	// Initialize included species list for this instance
	if err := birdnet.BuildRangeFilter(instance); err != nil {
		// Clean up the instance if species list initialization fails
		instance.Delete()
		return nil, fmt.Errorf("failed to initialize species list for instance %s: %w", id, err)
	}

	// Register the new instance
	birdnet.RegisterInstance(id, instance)
	instancesInitialized.Store(id, true)

	return instance, nil
}

// GetOrCreateBirdNETInstance gets an existing instance or creates a new one if it doesn't exist
func GetOrCreateBirdNETInstance(id string, settings *conf.Settings, modelPath, labelPath string) (*birdnet.BirdNET, error) {
	// Check if instance already exists
	instance := birdnet.GetInstance(id)
	if instance != nil {
		return instance, nil
	}

	// Create a new instance
	return InitializeBirdNETWithID(id, settings, modelPath, labelPath)
}

// Ensure global instance is initialized (for backward compatibility)
func EnsureGlobalInstanceInitialized(settings *conf.Settings) (*birdnet.BirdNET, error) {
	if err := initializeBirdNET(settings); err != nil {
		return nil, err
	}
	return bn, nil
}
