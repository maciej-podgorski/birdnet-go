package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"

	"github.com/tphakala/birdnet-go/internal/audio/file"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
	"github.com/tphakala/birdnet-go/internal/observation"
)

// FileAnalysis conducts an analysis of an audio file and outputs the results.
// It reads an audio file, analyzes it for bird sounds, and prints the results based on the provided configuration.
func FileAnalysis(settings *conf.Settings, ctx context.Context) error {
	// Initialize BirdNET interpreter
	if err := initializeBirdNET(settings); err != nil {
		return err
	}

	// Create file manager
	fileManager := file.NewManager(settings.Debug)

	// Validate the audio file
	if err := fileManager.ValidateFile(settings.Input.Path); err != nil {
		return err
	}

	// Get audio file information
	fileInfo, err := fileManager.GetFileInfo(settings.Input.Path)
	if err != nil {
		return fmt.Errorf("error getting audio info: %w", err)
	}

	notes, err := processAudioFile(settings, fileManager, &fileInfo, ctx)
	if err != nil {
		// Handle cancellation first
		if errors.Is(err, ErrAnalysisCanceled) {
			return nil
		}

		// For other errors with partial results, write them
		if len(notes) > 0 {
			fmt.Printf("\n\033[33m⚠️  Writing partial results before exiting due to error\033[0m\n")
			if writeErr := writeResults(settings, notes); writeErr != nil {
				return fmt.Errorf("analysis error: %w; failed to write partial results: %w", err, writeErr)
			}
		}
		return err
	}

	return writeResults(settings, notes)
}

// truncateString truncates a string to fit within maxLen, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatProgressLine formats the progress line to fit within the terminal width
func formatProgressLine(filename string, duration time.Duration, chunkCount, totalChunks int, avgRate float64, timeRemaining string, termWidth int) string {
	// Base format without filename (to calculate remaining space)
	baseFormat := fmt.Sprintf(" [%s] | \033[33m🔍 Analyzing chunk %d/%d\033[0m | \033[36m%.1f chunks/sec\033[0m %s",
		duration.Round(time.Second),
		chunkCount,
		totalChunks,
		avgRate,
		timeRemaining)

	// Calculate available space for filename
	// Account for emoji (📄) and color codes
	const colorCodesLen = 45 // Approximate length of all color codes
	availableSpace := termWidth - len(baseFormat) - colorCodesLen

	// Ensure minimum width
	if availableSpace < 10 {
		availableSpace = 10
	}

	// Truncate filename if needed
	truncatedFilename := truncateString(filename, availableSpace)

	// Return the complete formatted line
	return fmt.Sprintf("\r\033[K\033[37m📄 %s%s",
		truncatedFilename,
		baseFormat)
}

// monitorProgress starts a goroutine to monitor and display analysis progress
func monitorProgress(ctx context.Context, doneChan chan struct{}, filename string, duration time.Duration,
	totalChunks int, chunkCount *int64, startTime time.Time) {

	lastChunkCount := int64(0)
	lastProgressUpdate := startTime

	// Moving average window for chunks/sec calculation
	const windowSize = 10 // Number of samples to average
	chunkRates := make([]float64, 0, windowSize)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-doneChan:
			return
		case <-ticker.C:
			currentTime := time.Now()
			timeSinceLastUpdate := currentTime.Sub(lastProgressUpdate)

			// Get current chunk count atomically
			currentCount := atomic.LoadInt64(chunkCount)

			// Calculate current chunk rate
			chunksProcessed := currentCount - lastChunkCount
			currentRate := float64(chunksProcessed) / timeSinceLastUpdate.Seconds()

			// Update moving average
			if len(chunkRates) >= windowSize {
				// Remove oldest value
				chunkRates = chunkRates[1:]
			}
			chunkRates = append(chunkRates, currentRate)

			// Calculate average rate
			var avgRate float64
			if len(chunkRates) > 0 {
				sum := 0.0
				for _, rate := range chunkRates {
					sum += rate
				}
				avgRate = sum / float64(len(chunkRates))
			}

			// Update counters for next iteration
			lastChunkCount = currentCount
			lastProgressUpdate = currentTime

			// Get terminal width
			width, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				width = 80 // Default to 80 columns if we can't get terminal width
			}

			// Format and print the progress line
			fmt.Print(formatProgressLine(
				filename,
				duration,
				int(currentCount),
				totalChunks,
				avgRate,
				birdnet.EstimateTimeRemaining(startTime, int(currentCount), totalChunks),
				width,
			))
		}
	}
}

// processChunk handles the processing of a single audio chunk
func processChunk(ctx context.Context, chunk fileChunk, settings *conf.Settings,
	resultChan chan<- []datastore.Note, errorChan chan<- error) error {

	// BirdNET expects float32 data directly, no need to convert to PCM
	notes, err := bn.ProcessChunk(chunk.Data, chunk.FilePosition)
	if err != nil {
		// Block until we can send the error or context is cancelled
		select {
		case errorChan <- err:
			// Error successfully sent
		case <-ctx.Done():
			// If context is done while trying to send error, prioritize context error
			return ctx.Err()
		}
		return err
	}

	// Filter notes based on included species list
	var filteredNotes []datastore.Note
	for i := range notes {
		if settings.IsSpeciesIncluded(notes[i].ScientificName) {
			filteredNotes = append(filteredNotes, notes[i])
		}
	}

	// Block until we can send results or context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resultChan <- filteredNotes:
		return nil
	}
}

// startWorkers initializes and starts the worker goroutines for audio analysis
func startWorkers(ctx context.Context, numWorkers int, chunkChan chan fileChunk,
	resultChan chan []datastore.Note, errorChan chan error, settings *conf.Settings) {

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			if settings.Debug {
				fmt.Printf("DEBUG: Worker %d started\n", workerID)
			}
			defer func() {
				if settings.Debug {
					fmt.Printf("DEBUG: Worker %d finished\n", workerID)
				}
			}()

			for chunk := range chunkChan {
				select {
				case <-ctx.Done():
					select {
					case errorChan <- ctx.Err():
					default:
					}
					return
				default:
				}

				if err := processChunk(ctx, chunk, settings, resultChan, errorChan); err != nil {
					if settings.Debug {
						fmt.Printf("DEBUG: Worker %d encountered error: %v\n", workerID, err)
					}
					return
				}
			}
		}(i)
	}
}

// Define fileChunk type for internal use in this package
type fileChunk struct {
	Data         []float32
	FilePosition time.Time
}

// Calculate total number of chunks based on file info and settings
func calculateTotalChunks(fileInfo *file.Info, overlap float64) int {
	// Calculate chunk duration in samples (3 seconds)
	chunkDuration := 3.0 // seconds

	// Calculate stride (distance between chunk starts) in samples
	// Overlap is in seconds, so the step is (chunkDuration - overlap) seconds
	stepSamples := int((chunkDuration - overlap) * float64(fileInfo.SampleRate))

	if stepSamples <= 0 {
		// Invalid step size - overlap is too large
		return 1
	}

	// Initialize position to 0
	position := 0

	// Count chunks by simulating how the reader would process the file
	chunkCount := 0
	for position < fileInfo.TotalSamples {
		chunkCount++
		position += stepSamples
	}

	return chunkCount
}

// processAudioFile handles the processing of an audio file
func processAudioFile(settings *conf.Settings, fileManager file.Manager, fileInfo *file.Info, ctx context.Context) ([]datastore.Note, error) {
	// Calculate total chunks
	totalChunks := calculateTotalChunks(fileInfo, settings.BirdNET.Overlap)

	// Calculate audio duration from file info
	duration := fileInfo.Duration

	// Get filename and truncate if necessary
	filename := filepath.Base(settings.Input.Path)

	startTime := time.Now()
	var chunkCount int64 = 1

	// Set number of workers to 1
	numWorkers := 1

	if settings.Debug {
		// Calculate the values used in total chunk calculation for debugging
		chunkDuration := 3.0 // seconds
		stepSamples := int((chunkDuration - settings.BirdNET.Overlap) * float64(fileInfo.SampleRate))
		chunkSamples := int(chunkDuration * float64(fileInfo.SampleRate))

		// Simulate the chunk counting process
		position := 0
		simulatedChunkCount := 0

		// Print the steps of our calculation
		fmt.Printf("DEBUG: Calculating total chunks for file of %d samples:\n", fileInfo.TotalSamples)
		fmt.Printf("DEBUG: Using step size of %d samples (%.2f seconds with %.2f second overlap)\n",
			stepSamples, chunkDuration, settings.BirdNET.Overlap)

		// Only show first 5 and last 2 steps for long files
		maxStepsToShow := 7
		stepsShown := 0

		for position < fileInfo.TotalSamples {
			simulatedChunkCount++

			// Show detailed steps (limited to avoid spam)
			if simulatedChunkCount <= 5 ||
				(fileInfo.TotalSamples-position) <= stepSamples*2 {
				if stepsShown < maxStepsToShow {
					fmt.Printf("DEBUG: Chunk %d: position %d, advancing by %d samples\n",
						simulatedChunkCount, position, stepSamples)
					stepsShown++
				}
			} else if stepsShown == 5 {
				fmt.Printf("DEBUG: ... (skipping middle steps) ...\n")
				stepsShown++
			}

			position += stepSamples
		}

		fmt.Printf("DEBUG: Total chunks calculated: %d\n", totalChunks)

		// Standard debug information
		fmt.Printf("DEBUG: Starting analysis with %d total chunks and %d workers\n", totalChunks, numWorkers)
		fmt.Printf("DEBUG: Sample rate: %d Hz, Channels: %d, Bit depth: %d\n",
			fileInfo.SampleRate, fileInfo.NumChannels, fileInfo.BitDepth)
		fmt.Printf("DEBUG: Total samples: %d, Duration: %v, Overlap: %.2f seconds\n",
			fileInfo.TotalSamples, fileInfo.Duration, settings.BirdNET.Overlap)
		fmt.Printf("DEBUG: Chunk size: %.2f seconds (%d samples)\n",
			chunkDuration, chunkSamples)
		fmt.Printf("DEBUG: Chunk stride: %.2f seconds (%d samples)\n",
			(chunkDuration - settings.BirdNET.Overlap), stepSamples)
	}

	// Create buffered channels for processing
	chunkChan := make(chan fileChunk, 4)
	resultChan := make(chan []datastore.Note, 4)
	errorChan := make(chan error, 1)
	doneChan := make(chan struct{})
	eofChan := make(chan struct{}, 1) // Add new channel for signaling EOF

	var allNotes []datastore.Note

	// Create a single cancel function to coordinate shutdown
	var doneChanClosed sync.Once
	shutdown := func() {
		doneChanClosed.Do(func() {
			close(doneChan)
		})
	}
	defer shutdown()

	// Start worker goroutines
	startWorkers(ctx, numWorkers, chunkChan, resultChan, errorChan, settings)

	// Start progress monitoring goroutine
	go monitorProgress(ctx, doneChan, filename, duration, totalChunks, &chunkCount, startTime)

	// Start result collector goroutine
	var processingError error
	var processingErrorMutex sync.Mutex

	go func() {
		if settings.Debug {
			fmt.Println("DEBUG: Result collector started")
		}
		defer shutdown()

		for i := 1; i <= totalChunks; i++ {
			select {
			case <-ctx.Done():
				processingErrorMutex.Lock()
				processingError = ctx.Err()
				processingErrorMutex.Unlock()
				return
			case notes := <-resultChan:
				if settings.Debug {
					fmt.Printf("DEBUG: Received results for chunk #%d\n", atomic.LoadInt64(&chunkCount))
				}
				allNotes = append(allNotes, notes...)
				atomic.AddInt64(&chunkCount, 1)
				if atomic.LoadInt64(&chunkCount) > int64(totalChunks) {
					return
				}
			case err := <-errorChan:
				if settings.Debug {
					fmt.Printf("DEBUG: Collector received error: %v\n", err)
				}
				processingErrorMutex.Lock()
				processingError = err
				processingErrorMutex.Unlock()
				return
			case <-eofChan:
				// EOF reached, exit even if we haven't processed all expected chunks
				if settings.Debug {
					fmt.Printf("DEBUG: Received EOF signal, processed %d/%d chunks\n",
						atomic.LoadInt64(&chunkCount)-1, totalChunks)
				}
				return
			case <-time.After(5 * time.Second):
				if settings.Debug {
					fmt.Printf("DEBUG: Timeout waiting for chunk %d results\n", i)
					fmt.Printf("DEBUG: Current chunk count: %d/%d\n", atomic.LoadInt64(&chunkCount), totalChunks)
				}
				processingErrorMutex.Lock()
				currentCount := atomic.LoadInt64(&chunkCount)
				processingError = fmt.Errorf("timeout waiting for analysis results (processed %d/%d chunks)", currentCount, totalChunks)
				processingErrorMutex.Unlock()
				return
			}
		}
		if settings.Debug {
			fmt.Println("DEBUG: Collector finished normally")
		}
	}()

	// Initialize filePosition before the loop
	filePosition := time.Time{}

	// Create a processor function to handle chunks from the file.ProcessAudioFile method
	processor := func(chunk file.Chunk) error {
		if settings.Debug && atomic.LoadInt64(&chunkCount) == 1 {
			fmt.Printf("DEBUG: First chunk contains %d float32 samples\n", len(chunk.Data))
		}

		// Convert from file.Chunk to our internal fileChunk type
		internalChunk := fileChunk{
			Data:         chunk.Data,
			FilePosition: filePosition,
		}

		// Update position for next chunk - overlap is in seconds
		filePosition = filePosition.Add(time.Duration((3.0 - settings.BirdNET.Overlap) * float64(time.Second)))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunkChan <- internalChunk:
			return nil
		case <-doneChan:
			processingErrorMutex.Lock()
			err := processingError
			processingErrorMutex.Unlock()
			if err != nil {
				return err
			}
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout sending chunk to processing")
		}
	}

	// Process the audio file using the new file package
	chunkDuration := 3.0 // 3 seconds is the default chunk duration for BirdNET
	err := fileManager.ProcessAudioFile(ctx, settings.Input.Path, chunkDuration, settings.BirdNET.Overlap, processor)

	if settings.Debug {
		fmt.Println("DEBUG: Finished reading audio file")
	}
	close(chunkChan)
	eofChan <- struct{}{} // Signal that EOF was reached

	if settings.Debug {
		fmt.Println("DEBUG: Waiting for processing to complete")
	}
	<-doneChan // Wait for processing to complete

	// Handle errors and return results
	if err != nil {
		if settings.Debug {
			fmt.Printf("DEBUG: File processing error: %v\n", err)
		}
		if errors.Is(err, context.Canceled) {
			return allNotes, ErrAnalysisCanceled
		}
		return nil, fmt.Errorf("error processing audio: %w", err)
	}

	processingErrorMutex.Lock()
	err = processingError
	processingErrorMutex.Unlock()
	if err != nil {
		if settings.Debug {
			fmt.Printf("DEBUG: Processing error encountered: %v\n", err)
		}
		if errors.Is(err, context.Canceled) {
			return allNotes, ErrAnalysisCanceled
		}
		return allNotes, err
	}

	if settings.Debug {
		fmt.Println("DEBUG: Analysis completed successfully")
	}
	// Update final statistics
	totalTime := time.Since(startTime)
	avgChunksPerSec := float64(totalChunks) / totalTime.Seconds()

	// Get terminal width for final status line
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80 // Default to 80 columns if we can't get terminal width
	}

	// Format and print the final status line
	fmt.Print(formatProgressLine(
		filename,
		duration,
		totalChunks,
		totalChunks,
		avgChunksPerSec,
		fmt.Sprintf("in %s", birdnet.FormatDuration(totalTime)),
		width,
	))
	fmt.Println() // Add newline after completion

	return allNotes, nil
}

// writeResults writes the notes to the output file based on the configuration.
func writeResults(settings *conf.Settings, notes []datastore.Note) error {
	// Prepare the output file path if OutputDir is specified in the configuration.
	var outputFile string
	if settings.Output.File.Path != "" {
		// Safely concatenate file paths using filepath.Join to avoid cross-platform issues.
		outputFile = filepath.Join(settings.Output.File.Path, filepath.Base(settings.Input.Path))
	}

	// Output the notes based on the desired output type in the configuration.
	// If OutputType is not specified or if it's set to "table", output as a table format.
	if settings.Output.File.Type == "" || settings.Output.File.Type == "table" {
		if err := observation.WriteNotesTable(settings, notes, outputFile); err != nil {
			return fmt.Errorf("failed to write notes table: %w", err)
		}
	}
	// If OutputType is set to "csv", output as CSV format.
	if settings.Output.File.Type == "csv" {
		if err := observation.WriteNotesCsv(settings, notes, outputFile); err != nil {
			return fmt.Errorf("failed to write notes CSV: %w", err)
		}
	}
	return nil
}
