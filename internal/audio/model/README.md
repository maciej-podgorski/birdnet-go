# Audio Model Package

## Overview

The `model` package provides BirdNET model management functionality for the BirdNET-Go application. It serves as the interface between audio data processing and the BirdNET machine learning model for bird sound recognition.

This package is part of the larger [audio system](../README.md) which provides comprehensive audio processing capabilities for the BirdNet-Go application.

## Key Components

### Manager

- **Manager**: Central component for managing BirdNET model instances
  - Maps audio sources to specific model instances for multi-source analysis
  - Provides unified interface for audio analysis with BirdNET
  - Handles model configuration and initialization
  - Implements proper resource cleanup during shutdown
  - Ensures thread safety with read/write mutex locks
  - Outputs detailed prediction results when debug mode is enabled
  - Controls analysis timing based on configured overlap settings
  - Verifies data completeness before processing analysis

### Timing Control

- **Prediction Frequency Control**: Ensures analysis timing follows BirdNET's sliding window approach
  - Manages 3-second segments with configurable overlap
  - For example, with 1-second overlap, predictions run every 2 seconds
  - Provides consistent analysis timing across all audio sources
  - Prevents CPU overload from excessive predictions
  - Respects the configured `settings.BirdNET.Overlap` value

### Data Validation

- **Completeness Check**: Ensures analysis only runs on complete audio segments
  - Validates that incoming audio buffers contain enough samples for a full 3-second segment
  - Calculates expected buffer size based on sample rate, bit depth, and channel count
  - Skips analysis for incomplete data to prevent inaccurate predictions
  - Logs detailed information about skipped buffers when debug mode is enabled
  - Prevents analyzing partial segments that would produce unreliable results

### Utilities

- **Audio Conversion**: Functions for converting audio samples to the format required by BirdNET
  - Supports 16-bit, 24-bit, and 32-bit audio depth conversion
  - Converts PCM data to floating-point format for analysis

## Thread Safety

All components implement proper mutex locking to ensure thread-safe operations in concurrent environments:
- Model instance access is protected by read/write mutexes
- Source-to-model mapping is thread-safe
- Analysis operations are synchronized
- Timing control uses thread-safe access to last analysis times

## Debug Mode

When `settings.BirdNET.Debug` is enabled in configuration, the model manager will:

- Print detailed information about each prediction process
- Display the top 10 species results for each prediction with confidence scores
- Include source ID and timestamp in debug output
- Show analysis timing details including overlap and effective interval
- Report buffer sizes and indicate when buffers are skipped due to incomplete data
- Help with troubleshooting false negatives and positives

To enable debug mode, set `debug: true` under the `birdnet` section in your config.yaml:

```yaml
birdnet:
  debug: true
  sensitivity: 1.0
  threshold: 0.8
  # other settings...
```

## Usage Examples

```go
// Create a new model manager
manager := model.NewManager(settings)

// Configure a model for a specific audio source
err := manager.ConfigureModelForSource(
    "microphone1",    // sourceID
    "model1",         // modelID
    "/path/to/model", // modelPath
    "/path/to/labels" // labelPath
)

// Analyze audio data from a specific source
audioData := []byte{} // PCM audio data from capture
startTime := time.Now().UnixNano()
err = manager.Analyze("microphone1", audioData, startTime)

// Get a model instance for a specific source
modelInstance := manager.GetModelForSource("microphone1")

// Clean up resources when done
manager.Cleanup()
```

## Resource Management

- Proper tracking of model instances to prevent memory leaks
- Systematic cleanup of resources during shutdown
- Efficient model sharing across multiple audio sources

## Dependencies

- **github.com/tphakala/birdnet-go/internal/birdnet**: BirdNET model implementation
- **github.com/tphakala/birdnet-go/internal/conf**: Application configuration 