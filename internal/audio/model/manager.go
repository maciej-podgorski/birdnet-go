// Package model provides BirdNET model management functionality.
package model

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
)

// LastAnalysisTime tracks when each source was last analyzed
var LastAnalysisTime = struct {
	sync.RWMutex
	times map[string]time.Time
}{
	times: make(map[string]time.Time),
}

// Manager handles the management of BirdNET model instances.
type Manager struct {
	// Default BirdNET instance for fallback
	defaultInstance *birdnet.BirdNET

	// Maps source IDs to model IDs
	sourceToModelMap map[string]string

	// Maps model IDs to BirdNET instances
	modelInstances map[string]*birdnet.BirdNET

	// Settings for model creation
	settings *conf.Settings

	// Mutex for thread-safe access to maps
	mutex sync.RWMutex
}

// NewManager creates a new model manager.
func NewManager(settings *conf.Settings) *Manager {
	return &Manager{
		sourceToModelMap: make(map[string]string),
		modelInstances:   make(map[string]*birdnet.BirdNET),
		settings:         settings,
	}
}

// SetDefaultInstance sets the default BirdNET instance.
func (m *Manager) SetDefaultInstance(instance *birdnet.BirdNET) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.defaultInstance = instance

	// Also register as "default" in the model instances map
	if instance != nil {
		m.modelInstances["default"] = instance
	}
}

// AssignModelToSource assigns a model ID to a source ID.
func (m *Manager) AssignModelToSource(sourceID, modelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.sourceToModelMap[sourceID] = modelID
	log.Printf("📊 Assigned model %s to source %s", modelID, sourceID)
}

// RegisterModelInstance registers a BirdNET instance with a model ID.
func (m *Manager) RegisterModelInstance(modelID string, instance *birdnet.BirdNET) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.modelInstances[modelID] = instance
	log.Printf("📊 Registered model instance with ID %s", modelID)
}

// GetModelForSource returns the appropriate BirdNET instance for a source.
func (m *Manager) GetModelForSource(sourceID string) *birdnet.BirdNET {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check if this source has a specific model assigned
	if modelID, ok := m.sourceToModelMap[sourceID]; ok {
		// Check if we have this model instance registered
		if instance, ok := m.modelInstances[modelID]; ok {
			return instance
		}
	}

	// Fall back to the default instance
	return m.defaultInstance
}

// ListModelIDs returns a list of registered model IDs.
func (m *Manager) ListModelIDs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	ids := make([]string, 0, len(m.modelInstances))
	for id := range m.modelInstances {
		ids = append(ids, id)
	}
	return ids
}

// GetModelInstance returns a model instance by ID.
func (m *Manager) GetModelInstance(modelID string) *birdnet.BirdNET {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.modelInstances[modelID]
}

// GetDefaultModelInstance returns the default model instance.
func (m *Manager) GetDefaultModelInstance() *birdnet.BirdNET {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.defaultInstance
}

// ConfigureModelForSource creates and configures a BirdNET instance for a source.
func (m *Manager) ConfigureModelForSource(sourceID, modelID, modelPath, labelPath string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Map the source to the model ID
	m.sourceToModelMap[sourceID] = modelID

	// If the model instance already exists, we can just return
	if _, exists := m.modelInstances[modelID]; exists {
		log.Printf("📊 Source %s mapped to existing model instance %s", sourceID, modelID)
		return nil
	}

	// Otherwise, create a new instance
	// Create a copy of settings to avoid modifying the original
	instanceSettings := *m.settings

	// Override the model and label paths if provided
	if modelPath != "" {
		instanceSettings.BirdNET.ModelPath = modelPath
	}
	if labelPath != "" {
		instanceSettings.BirdNET.LabelPath = labelPath
	}

	// Create the new instance
	instance, err := birdnet.NewBirdNET(&instanceSettings)
	if err != nil {
		return fmt.Errorf("failed to create BirdNET instance for model %s: %w", modelID, err)
	}

	// Initialize species list for this instance
	if err := birdnet.BuildRangeFilter(instance); err != nil {
		// Clean up the instance if species list initialization fails
		instance.Delete()
		return fmt.Errorf("failed to initialize species list for model %s: %w", modelID, err)
	}

	// Register the instance with the registry
	birdnet.RegisterInstance(modelID, instance)

	// Store the instance locally
	m.modelInstances[modelID] = instance
	log.Printf("📊 Created new BirdNET instance %s for source %s", modelID, sourceID)

	return nil
}

// Cleanup cleans up all model instances.
func (m *Manager) Cleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Note: we don't actually clean up the instances here as they may be referenced
	// by the global registry. In a more complete implementation, we would track which
	// instances were created by this manager and only clean those up.

	// Clear the maps
	m.defaultInstance = nil
	m.sourceToModelMap = make(map[string]string)
	m.modelInstances = make(map[string]*birdnet.BirdNET)
}

// hasCompleteData checks if the provided data buffer has enough samples for a complete analysis
func (m *Manager) hasCompleteData(data []byte) bool {
	// Calculate required buffer size for a 3-second segment at the configured sample rate
	// BirdNET requires 3 seconds of audio data to analyze
	expectedSamples := 3 * conf.SampleRate // e.g., 3 seconds * 48000 Hz = 144000 samples
	bytesPerSample := conf.BitDepth / 8    // e.g., 16 bits = 2 bytes per sample
	expectedBytes := expectedSamples * bytesPerSample * conf.NumChannels

	// Check if we have enough data
	if len(data) < expectedBytes {
		return false
	}

	return true
}

// Analyze sends audio data to the appropriate BirdNET instance for analysis.
func (m *Manager) Analyze(sourceID string, data []byte, startTime int64) error {
	// Check if we have enough data for a complete analysis
	if !m.hasCompleteData(data) {
		if m.settings.BirdNET.Debug {
			log.Printf("⚠️ [DEBUG] Skipping analysis for source %s: incomplete data (%d bytes)",
				sourceID, len(data))
		}
		return nil // Skip this sample, not enough data
	}

	// Get the BirdNET instance for this source
	instance := m.GetModelForSource(sourceID)
	if instance == nil {
		return fmt.Errorf("no BirdNET instance available for source %s", sourceID)
	}

	// Convert audio data to float32
	sampleData, err := ConvertToFloat32(data, conf.BitDepth)
	if err != nil {
		return fmt.Errorf("error converting PCM data to float32: %w", err)
	}

	// Run BirdNET inference
	results, err := instance.Predict(sampleData)
	if err != nil {
		return fmt.Errorf("error predicting species: %w", err)
	}

	// Debug output when enabled
	if m.settings.BirdNET.Debug {
		timestamp := time.Unix(startTime, 0).Format("15:04:05")
		log.Printf("🔍 [DEBUG] BirdNET Prediction Results at %s for source %s:", timestamp, sourceID)
		log.Printf("  Data size: %d bytes (expected min: %d bytes)",
			len(data), 3*conf.SampleRate*(conf.BitDepth/8)*conf.NumChannels)
		for i, result := range results {
			if i < 10 { // Print top 10 results
				log.Printf("  %d. %s: %.6f", i+1, result.Species, result.Confidence)
			}
		}

		// Log the timing information to help debug prediction frequency
		bufferDuration := 3 * time.Second
		overlapDuration := time.Duration(m.settings.BirdNET.Overlap * float64(time.Second))
		effectiveInterval := bufferDuration - overlapDuration
		log.Printf("🔍 [DEBUG] Analysis timing: overlap=%v, effective interval=%v",
			overlapDuration, effectiveInterval)
	}

	// Send results to the BirdNET results queue
	resultsMessage := birdnet.Results{
		StartTime: time.Unix(startTime, 0),
		PCMdata:   data,
		Results:   results,
		Source:    sourceID,
	}

	// Create a deep copy of the Results struct
	copyToSend := resultsMessage.Copy()

	// Send the results to the queue
	select {
	case birdnet.ResultsQueue <- copyToSend:
		// Results enqueued successfully
	default:
		log.Println("❌ Results queue is full!")
		// Queue is full
	}

	return nil
}
