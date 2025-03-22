package ffmpeg

import (
	"fmt"
	"strings"
)

// StreamFormat defines audio format parameters
type StreamFormat struct {
	SampleRate int
	Channels   int
	BitDepth   int
}

// StreamProtocol defines the transport protocol
type StreamProtocol string

const (
	ProtocolTCP  StreamProtocol = "tcp"
	ProtocolUDP  StreamProtocol = "udp"
	ProtocolHTTP StreamProtocol = "http"
	ProtocolHLS  StreamProtocol = "hls"
)

// StreamSource defines a stream source for FFmpeg
type StreamSource struct {
	URL      string
	Protocol StreamProtocol
}

// CommandBuilder builds FFmpeg commands
type CommandBuilder struct {
	ffmpegPath string
	input      string
	inputOpts  []string
	output     string
	outputOpts []string
	format     StreamFormat
	globalOpts []string
}

// NewCommandBuilder creates a new command builder with the given FFmpeg path
func NewCommandBuilder(ffmpegPath string) *CommandBuilder {
	return &CommandBuilder{
		ffmpegPath: ffmpegPath,
		inputOpts:  make([]string, 0),
		outputOpts: make([]string, 0),
		globalOpts: []string{"-hide_banner", "-loglevel", "error"},
	}
}

// WithFormat sets the audio format
func (b *CommandBuilder) WithFormat(format StreamFormat) *CommandBuilder {
	b.format = format
	return b
}

// WithInputURL sets the input URL
func (b *CommandBuilder) WithInputURL(source StreamSource) *CommandBuilder {
	// Handle different protocols
	switch source.Protocol {
	case ProtocolTCP, ProtocolUDP:
		b.inputOpts = append(b.inputOpts, "-rtsp_transport", string(source.Protocol))
	case ProtocolHLS:
		// HLS specific options if needed
		b.inputOpts = append(b.inputOpts, "-live_start_index", "-1")
	}

	b.input = source.URL
	return b
}

// WithInputFile sets an input file
func (b *CommandBuilder) WithInputFile(filePath string) *CommandBuilder {
	b.input = filePath
	return b
}

// WithInputPipe configures input from stdin
func (b *CommandBuilder) WithInputPipe() *CommandBuilder {
	b.input = "-"
	return b
}

// WithOutputPipe configures output to stdout
func (b *CommandBuilder) WithOutputPipe() *CommandBuilder {
	b.output = "pipe:1"
	return b
}

// WithOutputFile sets an output file
func (b *CommandBuilder) WithOutputFile(filePath string) *CommandBuilder {
	b.output = filePath
	return b
}

// WithOutputFormat sets the output format (codec)
func (b *CommandBuilder) WithOutputFormat(format string) *CommandBuilder {
	b.outputOpts = append(b.outputOpts, "-f", format)
	return b
}

// WithOutputCodec sets the output audio codec
func (b *CommandBuilder) WithOutputCodec(codec string) *CommandBuilder {
	b.outputOpts = append(b.outputOpts, "-c:a", codec)
	return b
}

// WithOutputBitrate sets the output bitrate
func (b *CommandBuilder) WithOutputBitrate(bitrate string) *CommandBuilder {
	b.outputOpts = append(b.outputOpts, "-b:a", bitrate)
	return b
}

// WithCustomOption adds a custom option
func (b *CommandBuilder) WithCustomOption(name, value string) *CommandBuilder {
	if value == "" {
		b.globalOpts = append(b.globalOpts, name)
	} else {
		b.globalOpts = append(b.globalOpts, name, value)
	}
	return b
}

// DisableVideo disables video processing
func (b *CommandBuilder) DisableVideo() *CommandBuilder {
	b.globalOpts = append(b.globalOpts, "-vn")
	return b
}

// Build builds the complete command arguments
func (b *CommandBuilder) Build() []string {
	args := make([]string, 0)

	// Add global options
	args = append(args, b.globalOpts...)

	// Add input options
	args = append(args, b.inputOpts...)

	// Add input
	args = append(args, "-i", b.input)

	// Add format options based on the format
	if b.format.SampleRate > 0 {
		args = append(args, "-ar", fmt.Sprintf("%d", b.format.SampleRate))
	}

	if b.format.Channels > 0 {
		args = append(args, "-ac", fmt.Sprintf("%d", b.format.Channels))
	}

	// Convert bit depth to FFmpeg format specifier
	if b.format.BitDepth > 0 {
		format := getFormatForBitDepth(b.format.BitDepth)
		if format != "" && !containsOption(args, "-f") {
			args = append(args, "-f", format)
		}
	}

	// Add output options
	args = append(args, b.outputOpts...)

	// Force overwrite
	args = append(args, "-y")

	// Add output
	args = append(args, b.output)

	return args
}

// BuildCommand returns the command string (for debugging)
func (b *CommandBuilder) BuildCommand() string {
	args := b.Build()
	return fmt.Sprintf("%s %s", b.ffmpegPath, strings.Join(args, " "))
}

// getFormatForBitDepth returns the FFmpeg format for a given bit depth
func getFormatForBitDepth(bitDepth int) string {
	switch bitDepth {
	case 16:
		return "s16le"
	case 24:
		return "s24le"
	case 32:
		return "s32le"
	default:
		return "s16le" // Default to 16-bit
	}
}

// containsOption checks if the option is already in the arguments
func containsOption(args []string, option string) bool {
	for _, arg := range args {
		if arg == option {
			return true
		}
	}
	return false
}
