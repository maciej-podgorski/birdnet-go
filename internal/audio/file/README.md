# File Package

## Overview

The `file` package provides functionality for working with audio files in the BirdNET-Go application. It offers a structured, interface-based approach to reading, validating, and processing audio files with consistent error handling across supported formats, as well as audio export capabilities.

This package is designed to be cross-platform (Linux, macOS, and Windows) and provides a unified interface for working with different audio file formats.

## Package Structure

The file system follows an interface-based design:

### Interfaces

- **Reader**: Interface for reading audio files and extracting data
  - Handles file opening, validation, metadata extraction, and chunk reading
  - Provides methods for processing files with custom processors
  - Manages proper resource cleanup

- **ReaderFactory**: Factory interface for creating appropriate readers
  - Creates format-specific readers based on file extensions
  - Encapsulates reader creation logic for easy extension

- **Manager**: High-level interface for audio file operations
  - Provides methods for file validation, info extraction, and processing
  - Coordinates between file formats through the ReaderFactory
  - Handles audio file conversion (PCM conversion feature planned)

### Implementations

- **StandardReaderFactory**: Default implementation of ReaderFactory
  - Creates type-specific readers (WAVReader, FLACReader) based on file extension
  - Handles unsupported format errors

- **StandardManager**: Default implementation of Manager
  - Uses ReaderFactory to create appropriate readers
  - Provides file validation, info extraction, and processing functionality
  - Implements chunk reading and audio conversion operations

- **WAVReader**: WAVE format implementation of Reader
  - Handles .wav file reading and metadata extraction
  - Processes audio data in configurable chunks

- **FLACReader**: FLAC format implementation of Reader
  - Handles .flac file reading and metadata extraction
  - Processes audio data in configurable chunks

### Types

- **Format**: Enum type for audio formats (PCM, WAV, FLAC)
- **Info**: Structure containing audio file metadata (sample rate, channels, bits per sample, duration)
- **Chunk**: Structure representing a chunk of audio data with timing information

### Export Functions

- **ExportAudioWithFFmpeg**: Exports PCM data to specified audio formats using FFmpeg
  - Supports various formats like MP3, FLAC, and WAV
  - Uses cross-platform FFmpeg binary detection
  - Configurable bitrate and audio parameters

- **SavePCMDataToWAV**: Saves PCM data to a WAV file with default parameters
  - 48000 Hz sample rate, 16-bit, mono channel

- **SavePCMToWAV**: Saves PCM data to a WAV file with custom parameters
  - Configurable sample rate, bit depth, and channel count

- **GetFileExtension**: Returns the appropriate file extension for a given format type

## Key Features

- **Format Support**: Handles common audio formats (.wav, .flac, .mp3)
- **Metadata Extraction**: Gets sample rate, channel count, bit depth, and duration
- **Chunk Processing**: Extracts audio in fixed-duration chunks with configurable overlap
- **Custom Processors**: Process audio chunks with custom callback functions
- **Validation**: Verifies audio files for format compliance and correctness
- **Error Handling**: Proper error wrapping and descriptive error messages
- **Resource Management**: Ensures proper cleanup of file handles
- **Audio Export**: Converts raw PCM data to various audio formats

## Usage Examples

### Getting File Information

```go
// Create a file manager
fileManager := file.NewManager(false) // debug = false

// Get file metadata
info, err := fileManager.GetFileInfo("path/to/audio.wav")
if err != nil {
    log.Fatal("Failed to get file info:", err)
}

fmt.Printf("Sample Rate: %d Hz\n", info.SampleRate)
fmt.Printf("Channels: %d\n", info.Channels)
fmt.Printf("Bits Per Sample: %d\n", info.BitsPerSample)
fmt.Printf("Duration: %.2f seconds\n", info.Duration)
```

### Validating an Audio File

```go
// Create a file manager
fileManager := file.NewManager(false)

// Validate the file
if err := fileManager.ValidateFile("path/to/audio.flac"); err != nil {
    log.Fatal("Invalid audio file:", err)
}

fmt.Println("File is valid!")
```

### Processing an Audio File

```go
// Create a file manager
fileManager := file.NewManager(false)

// Define a processor function
processor := func(chunk file.Chunk) error {
    fmt.Printf("Processing chunk at %.2f seconds, duration: %.2f\n", 
        chunk.StartTime, chunk.Duration)
    
    // Process audio data in chunk.Data
    // ...
    
    return nil
}

// Process the file with 3-second chunks and 1.5 second overlap
ctx := context.Background()
err := fileManager.ProcessAudioFile(ctx, "path/to/audio.wav", 3.0, 1.5, processor)
if err != nil {
    log.Fatal("Failed to process audio file:", err)
}
```

### Reading All Chunks

```go
// Create a file manager
fileManager := file.NewManager(false)

// Read all chunks from a file
chunks, err := fileManager.ReadChunks("path/to/audio.wav", 3.0, 0.0)
if err != nil {
    log.Fatal("Failed to read chunks:", err)
}

fmt.Printf("Read %d chunks\n", len(chunks))
for i, chunk := range chunks {
    fmt.Printf("Chunk %d: %.2f seconds, %d samples\n", 
        i, chunk.StartTime, len(chunk.Data)/4) // Assuming 16-bit stereo
}
```

### Exporting Audio to Different Formats

```go
// Export PCM data to WAV format
if err := file.SavePCMDataToWAV("output/recording.wav", pcmData); err != nil {
    log.Fatal("Failed to save WAV file:", err)
}

// Export PCM data to a custom format (MP3, FLAC, etc.) using FFmpeg
audioSettings := &conf.AudioSettings{
    FfmpegPath: "/usr/bin/ffmpeg", // Optional, will find automatically if empty
    Export: struct {
        Debug     bool   
        Enabled   bool   
        Path      string 
        Type      string 
        Bitrate   string 
        Retention struct {
            Debug    bool   
            Policy   string 
            MaxAge   string 
            MaxUsage string 
            MinClips int    
        }
    }{
        Type: "mp3",
        Bitrate: "192k",
    },
}

if err := file.ExportAudioWithFFmpeg(pcmData, "output/recording.mp3", audioSettings); err != nil {
    log.Fatal("Failed to export audio:", err)
}
```

## Error Handling

The package implements proper error wrapping with context:

```go
// Examples of error handling in the package
return fmt.Errorf("failed to open file: %w", err)
return fmt.Errorf("error validating file: %w", err)
return fmt.Errorf("invalid audio file %s", filepath.Base(filePath))
```

## Cross-Platform Compatibility

The package ensures cross-platform compatibility by:
- Using path/filepath for cross-platform path handling
- Using os.MkdirAll for creating directory structures on any OS
- Utilizing conf.GetFfmpegBinaryName() to get the correct FFmpeg binary name for each OS
- Implementing platform-neutral file operations

## Dependencies

- Standard Go library components:
  - context: For cancellation and timeout handling
  - io: For EOF detection and general I/O operations
  - os: For file operations
  - path/filepath: For cross-platform path handling
  - strings: For string manipulation
  - os/exec: For executing FFmpeg commands

- External dependencies:
  - github.com/go-audio/audio: For audio buffer handling
  - github.com/go-audio/wav: For WAV encoding/decoding

## Future Enhancements

- **MP3 Reader**: Add direct MP3 file reading support
- **PCM Conversion**: Implement PCM conversion functionality
- **Additional Formats**: Support for AAC, OGG, and other formats
- **Metadata Extraction**: Enhanced metadata extraction capabilities
- **Stream Conversion**: Convert between file formats with quality options 