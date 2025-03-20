package myaudio

import (
	"encoding/binary"
	"math"
)

// calculateAudioLevel calculates the RMS (Root Mean Square) of the audio samples
// and returns an AudioLevelData struct with the level and clipping status
func calculateAudioLevel(samples []byte, source, name string) AudioLevelData {
	// If there are no samples, return zero level and no clipping
	if len(samples) == 0 {
		return AudioLevelData{Level: 0, Clipping: false, Source: source, Name: name}
	}

	// Ensure we have an even number of bytes (16-bit samples)
	if len(samples)%2 != 0 {
		// Truncate to even number of bytes
		samples = samples[:len(samples)-1]
	}

	var sum float64
	sampleCount := len(samples) / 2 // 2 bytes per sample for 16-bit audio
	isClipping := false
	maxSample := float64(0)

	// Iterate through samples, calculating sum of squares and checking for clipping
	for i := 0; i < len(samples); i += 2 {
		if i+1 >= len(samples) {
			break
		}

		// Convert two bytes to a 16-bit sample
		sample := int16(binary.LittleEndian.Uint16(samples[i : i+2]))
		sampleAbs := math.Abs(float64(sample))
		sum += sampleAbs * sampleAbs

		// Keep track of the maximum sample value
		if sampleAbs > maxSample {
			maxSample = sampleAbs
		}

		// Check for clipping (maximum positive or negative 16-bit value)
		if sample == 32767 || sample == -32768 {
			isClipping = true
		}
	}

	// If we ended up with no samples, return zero level and no clipping
	if sampleCount == 0 {
		return AudioLevelData{Level: 0, Clipping: false, Source: source, Name: name}
	}

	// Calculate Root Mean Square (RMS)
	rms := math.Sqrt(sum / float64(sampleCount))

	// Convert RMS to decibels
	// 32768 is max value for 16-bit audio
	db := 20 * math.Log10(rms/32768.0)

	// Scale decibels to 0-100 range
	// Adjust the range to make it more sensitive
	scaledLevel := (db + 60) * (100.0 / 50.0)

	// If the audio is clipping, ensure the level is at or near 100
	if isClipping {
		scaledLevel = math.Max(scaledLevel, 95)
	}

	// Clamp the value between 0 and 100
	if scaledLevel < 0 {
		scaledLevel = 0
	} else if scaledLevel > 100 {
		scaledLevel = 100
	}

	// Return the calculated audio level data
	return AudioLevelData{
		Level:    int(scaledLevel),
		Clipping: isClipping,
		Source:   source,
		Name:     name,
	}
}
