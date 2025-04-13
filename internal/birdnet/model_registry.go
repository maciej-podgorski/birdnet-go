// model_registry.go contains information about supported models and their properties
package birdnet

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ModelInfo represents metadata about a BirdNET model
type ModelInfo struct {
	ID               string   // Unique identifier for the model
	Name             string   // User-friendly name
	Description      string   // Description of the model
	SupportedLocales []string // List of supported locale codes
	DefaultLocale    string   // Default locale if none is specified
	NumSpecies       int      // Number of species in the model
	CustomPath       string   // Path to custom model file, if any
}

// Predefined supported models
var supportedModels = map[string]ModelInfo{
	"BirdNET_GLOBAL_6K_V2.4": {
		ID:          "BirdNET_GLOBAL_6K_V2.4",
		Name:        "BirdNET GLOBAL 6K V2.4",
		Description: "Global model with 6523 species",
		SupportedLocales: []string{"af", "ar", "bg", "ca", "cs", "da", "de", "el", "en", "en-uk", "es",
			"et", "fi", "fr", "he", "hr", "hu", "id", "is", "it", "ja", "ko", "lt", "lv", "ml", "nl",
			"no", "pl", "pt", "pt-br", "pt-pt", "ro", "ru", "sk", "sl", "sr", "sv", "th", "tr", "uk", "zh"},
		DefaultLocale: "en",
		NumSpecies:    6523,
	},
}

// DetermineModelInfo identifies the model type from a file path or model identifier
func DetermineModelInfo(modelPathOrID string) (ModelInfo, error) {
	// Check if it's a known model ID
	if info, exists := supportedModels[modelPathOrID]; exists {
		return info, nil
	}

	// If it's a path to a custom model file
	if strings.HasSuffix(modelPathOrID, ".tflite") {
		// Try to determine model type from filename
		baseName := filepath.Base(modelPathOrID)

		// Check if it matches known patterns
		for id, info := range supportedModels {
			if strings.Contains(baseName, id) {
				// Clone the model info but mark it as custom
				customInfo := info
				customInfo.CustomPath = modelPathOrID
				return customInfo, nil
			}
		}

		// If we couldn't identify it, create a generic custom model entry
		return ModelInfo{
			ID:          "Custom",
			Name:        "Custom Model",
			Description: fmt.Sprintf("Custom model from %s", baseName),
			CustomPath:  modelPathOrID,
		}, nil
	}

	return ModelInfo{}, fmt.Errorf("unrecognized model: %s", modelPathOrID)
}

// IsLocaleSupported checks if a locale is supported by the given model
func IsLocaleSupported(modelInfo *ModelInfo, locale string) bool {
	// If it's a custom model with no specified locales, assume all are supported
	if len(modelInfo.SupportedLocales) == 0 {
		return true
	}

	for _, supported := range modelInfo.SupportedLocales {
		if supported == locale {
			return true
		}
	}

	return false
}
