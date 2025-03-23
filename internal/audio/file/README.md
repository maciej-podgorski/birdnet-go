# File Package

## Overview

The `file` package provides functionality for working with audio files in the BirdNET-Go application. It offers a structured, interface-based approach to reading, validating, and processing audio files with consistent error handling across supported formats.

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

## Key Features

- **Format Support**: Handles common audio formats (.wav, .flac)
- **Metadata Extraction**: Gets sample rate, channel count, bit depth, and duration
- **Chunk Processing**: Extracts audio in fixed-duration chunks with configurable overlap
- **Custom Processors**: Process audio chunks with custom callback functions
- **Validation**: Verifies audio files for format compliance and correctness
- **Error Handling**: Proper error wrapping and descriptive error messages
- **Resource Management**: Ensures proper cleanup of file handles

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

## Error Handling

The package implements proper error wrapping with context:

```go
// Examples of error handling in the package
return fmt.Errorf("failed to open file: %w", err)
return fmt.Errorf("error validating file: %w", err)
return fmt.Errorf("invalid audio file %s", filepath.Base(filePath))
```

## Future Enhancements

- **MP3 Support**: Add support for MP3 file format
- **PCM Conversion**: Implement PCM conversion functionality
- **Additional Formats**: Support for AAC, OGG, and other formats
- **Metadata Extraction**: Enhanced metadata extraction capabilities
- **Stream Conversion**: Convert between file formats with quality options

## Dependencies

- Standard Go library components:
  - context: For cancellation and timeout handling
  - io: For EOF detection and general I/O operations
  - os: For file operations
  - path/filepath: For cross-platform path handling
  - strings: For string manipulation

No external dependencies are required for basic functionality. 