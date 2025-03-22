package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ExportFormat defines the format for audio export
type ExportFormat struct {
	Type    string // "flac", "mp3", "aac", "alac", "opus"
	Bitrate string // e.g., "192k"
}

// ExportOptions defines options for audio export
type ExportOptions struct {
	FFmpegPath   string
	Format       ExportFormat
	StreamFormat StreamFormat
	OutputPath   string
	Timeout      time.Duration // Maximum time to wait for export to complete
}

// Export exports PCM audio data to a file in the specified format
func Export(ctx context.Context, data []byte, opts ExportOptions) error {
	// Validate options
	if opts.FFmpegPath == "" {
		return fmt.Errorf("FFmpeg path not specified")
	}

	if opts.OutputPath == "" {
		return fmt.Errorf("output path not specified")
	}

	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute // Default timeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(opts.OutputPath)
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create a temporary file path for atomic operation
	tempPath := opts.OutputPath + ".temp"

	// Create command builder
	builder := NewCommandBuilder(opts.FFmpegPath)

	// Configure input
	builder.WithInputPipe().WithFormat(opts.StreamFormat)

	// Configure output
	builder.WithOutputFile(tempPath)

	// Set output format and codec based on the export format
	codec := getEncoder(opts.Format.Type)
	builder.WithOutputCodec(codec)

	// Set output container format
	container := getOutputFormat(opts.Format.Type)
	builder.WithOutputFormat(container)

	// Set bitrate with limits
	bitrate := getLimitedBitrate(opts.Format.Type, opts.Format.Bitrate)
	builder.WithOutputBitrate(bitrate)

	// Start the FFmpeg process
	process, err := Start(ctx, ProcessOptions{
		FFmpegPath: opts.FFmpegPath,
		Args:       builder.Build(),
		InputData:  data,
	})
	if err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Wait for the process to complete
	if err := process.Stop(); err != nil {
		// Clean up temporary file
		os.Remove(tempPath)

		if stderr := process.StderrOutput(); stderr != "" {
			return fmt.Errorf("FFmpeg export failed: %w\nStderr: %s", err, stderr)
		}
		return fmt.Errorf("FFmpeg export failed: %w", err)
	}

	// Rename temporary file to final output file (atomic operation)
	if err := os.Rename(tempPath, opts.OutputPath); err != nil {
		// Clean up temporary file
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// getEncoder returns the appropriate codec based on format
func getEncoder(format string) string {
	switch format {
	case "flac":
		return "flac"
	case "alac":
		return "alac"
	case "opus":
		return "libopus"
	case "aac":
		return "aac"
	case "mp3":
		return "libmp3lame"
	default:
		return format
	}
}

// getOutputFormat returns the appropriate container format
func getOutputFormat(format string) string {
	switch format {
	case "flac":
		return "flac"
	case "alac":
		return "ipod" // ALAC uses the iPod container format
	case "opus":
		return "opus"
	case "aac":
		return "mp4" // AAC typically uses the iPod/MP4 container format
	case "mp3":
		return "mp3"
	default:
		return format
	}
}

// getLimitedBitrate limits the bitrate based on the format
func getLimitedBitrate(format, requestedBitrate string) string {
	switch format {
	case "opus":
		if requestedBitrate > "256k" {
			return "256k"
		}
	case "mp3":
		if requestedBitrate > "320k" {
			return "320k"
		}
	}
	return requestedBitrate
}
