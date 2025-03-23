package file

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// Standard audio constants
const (
	DefaultSampleRate = 48000
	DefaultBitDepth   = 16
	DefaultChannels   = 1
)

// GetFileExtension returns the file extension for a given format type.
func GetFileExtension(formatType string) string {
	switch formatType {
	case "wav":
		return "wav"
	case "flac":
		return "flac"
	case "mp3":
		return "mp3"
	case "aac", "m4a":
		return "m4a"
	case "opus":
		return "opus"
	case "ogg":
		return "ogg"
	default:
		return formatType
	}
}

// SavePCMDataToWAV saves the given PCM data as a WAV file at the specified filePath.
func SavePCMDataToWAV(filePath string, pcmData []byte) error {
	return SavePCMToWAV(filePath, pcmData, DefaultSampleRate, DefaultBitDepth, DefaultChannels)
}

// SavePCMToWAV saves PCM data to a WAV file with custom parameters
func SavePCMToWAV(filePath string, pcmData []byte, sampleRate, bitDepth, channels int) error {
	// Create the directory structure if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Open a new file for writing.
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close() // Ensure file closure on function exit.

	// Create a new WAV encoder with the specified format settings.
	enc := wav.NewEncoder(outFile, sampleRate, bitDepth, channels, 1)

	// Convert the byte slice to a slice of integer samples.
	intSamples := byteSliceToInts(pcmData)

	// Write the integer samples to the WAV file.
	if err := enc.Write(&audio.IntBuffer{
		Data:   intSamples,
		Format: &audio.Format{SampleRate: sampleRate, NumChannels: channels},
	}); err != nil {
		return fmt.Errorf("failed to write to WAV encoder: %w", err)
	}

	// Close the WAV encoder, which finalizes the file format.
	return enc.Close()
}

// byteSliceToInts converts a byte slice to a slice of integers.
// Each pair of bytes is treated as a single 16-bit sample.
func byteSliceToInts(pcmData []byte) []int {
	var samples []int
	buf := bytes.NewBuffer(pcmData)

	// Read each 16-bit sample from the byte buffer and store it as an int.
	for {
		var sample int16
		if err := binary.Read(buf, binary.LittleEndian, &sample); err != nil {
			break // Exit loop on read error (e.g., end of buffer).
		}
		samples = append(samples, int(sample))
	}

	return samples
}
