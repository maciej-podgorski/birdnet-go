package file

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/tphakala/birdnet-go/internal/audio/ffmpeg"
	"github.com/tphakala/birdnet-go/internal/conf"
)

// ExportAudioWithFFmpeg exports PCM data to the specified audio format using FFmpeg.
func ExportAudioWithFFmpeg(pcmData []byte, outputPath string, audioSettings *conf.AudioSettings) error {
	// Get FFmpeg path from config or find it using conf utils
	ffmpegPath := audioSettings.FfmpegPath
	if ffmpegPath == "" {
		var err error
		ffmpegBinaryName := conf.GetFfmpegBinaryName()
		ffmpegPath, err = exec.LookPath(ffmpegBinaryName)
		if err != nil {
			return fmt.Errorf("FFmpeg not found: %w", err)
		}
	}

	// Create export options
	exportFormat := ffmpeg.ExportFormat{
		Type:    audioSettings.Export.Type,
		Bitrate: audioSettings.Export.Bitrate,
	}

	// Set stream format with default BirdNET values
	streamFormat := ffmpeg.StreamFormat{
		SampleRate: 48000, // Default BirdNET sample rate
		Channels:   1,     // Default mono
		BitDepth:   16,    // Default 16-bit
	}

	// Create export options
	exportOptions := &ffmpeg.ExportOptions{
		FFmpegPath:   ffmpegPath,
		Format:       exportFormat,
		StreamFormat: streamFormat,
		OutputPath:   outputPath,
		Timeout:      30 * time.Second, // Default timeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), exportOptions.Timeout)
	defer cancel()

	// Export the audio data
	return ffmpeg.Export(ctx, pcmData, exportOptions)
}
