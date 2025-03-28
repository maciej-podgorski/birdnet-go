package file

import (
	"context"
	"fmt"
	"math"
)

// StreamingResampler provides an optimized resampler for streaming audio
type StreamingResampler struct {
	originalRate int
	targetRate   int
	ratio        float64

	// Filter parameters
	filter     []float64
	filterLen  int
	filterHalf int

	// Optimized polyphase filter banks for common ratios
	polyphaseFilters [][]float64
	numPhases        int

	// Precomputed tables
	sincTable   []float64
	windowTable []float64

	// Buffer for history samples needed for filtering across chunk boundaries
	history []float32

	// Performance optimizations
	isIntegerRatio bool
	integerRatio   int
	commonRatio    bool

	// Scratch buffer for resampling to avoid allocations per chunk
	scratchBuffer []float32
}

// NewStreamingResampler creates a new optimized streaming resampler
func NewStreamingResampler(originalRate, targetRate int) *StreamingResampler {
	r := &StreamingResampler{
		originalRate: originalRate,
		targetRate:   targetRate,
		ratio:        float64(targetRate) / float64(originalRate),
	}

	// Check if we have an integer or simple fractional ratio
	r.checkRatioProperties()

	// Determine filter parameters - focused on upsampling for BirdNET
	if originalRate < targetRate {
		filterQuality := 0.97
		cutoffFreq := float64(originalRate) / 2.0
		normalizedCutoff := cutoffFreq / float64(targetRate)

		// Calculate filter length based on quality and transition bandwidth
		r.filterLen = int(math.Ceil(4.0 * filterQuality * float64(targetRate) / cutoffFreq))
		if r.filterLen%2 == 0 {
			r.filterLen++
		}
		r.filterHalf = r.filterLen / 2

		// Precompute general filter
		r.filter = designLowpassFilter(normalizedCutoff, r.filterLen)

		// For 3x upsampling (16kHz -> 48kHz), precompute phase filters
		if r.isIntegerRatio && r.integerRatio == 3 {
			r.precomputePolyphaseFilters(3)
		} else if r.originalRate == 32000 && r.targetRate == 48000 {
			// 32kHz -> 48kHz (3:2 ratio)
			r.precomputePolyphaseFilters(3) // 3 phases for 3:2 ratio
		}

		// Precompute sinc and window tables for faster filter generation
		r.precomputeTables()
	} else {
		// Downsampling (similar but with adjusted cutoff)
		filterQuality := 0.97
		cutoffFreq := float64(targetRate) / 2.0
		normalizedCutoff := cutoffFreq / float64(originalRate)

		r.filterLen = int(math.Ceil(4.0 * filterQuality * float64(originalRate) / cutoffFreq))
		if r.filterLen%2 == 0 {
			r.filterLen++
		}
		r.filterHalf = r.filterLen / 2

		r.filter = designLowpassFilter(normalizedCutoff, r.filterLen)

		// For common integer downsampling ratios, precompute phase filters
		if r.isIntegerRatio && r.integerRatio <= 8 {
			r.precomputePolyphaseFilters(1) // Only one phase needed for downsampling
		}

		r.precomputeTables()
	}

	// Allocate history buffer
	r.history = make([]float32, r.filterHalf)

	return r
}

// checkRatioProperties analyzes the resampling ratio for optimization opportunities
func (r *StreamingResampler) checkRatioProperties() {
	// Check if we have an integer ratio
	intRatio := int(r.ratio + 0.5)
	if math.Abs(float64(intRatio)-r.ratio) < 1e-10 {
		r.isIntegerRatio = true
		r.integerRatio = intRatio
	}

	// Check if we have a common ratio
	switch {
	case r.originalRate == 16000 && r.targetRate == 48000:
		r.commonRatio = true // 3:1 ratio
	case r.originalRate == 32000 && r.targetRate == 48000:
		r.commonRatio = true // 3:2 ratio
	case r.originalRate == 22050 && r.targetRate == 44100:
		r.commonRatio = true // 2:1 ratio
	case r.originalRate == 44100 && r.targetRate == 48000:
		r.commonRatio = true // 48:41 ratio (approximately)
	}
}

// precomputePolyphaseFilters creates optimized filter banks for different phases
func (r *StreamingResampler) precomputePolyphaseFilters(phases int) {
	r.numPhases = phases
	r.polyphaseFilters = make([][]float64, phases)

	if r.originalRate < r.targetRate {
		// Upsampling
		for i := 0; i < phases; i++ {
			// Create a phase-specific filter
			r.polyphaseFilters[i] = make([]float64, r.filterLen)

			// For each phase, apply a fractional delay to the filter
			phaseOffset := float64(i) / float64(phases)

			for j := 0; j < r.filterLen; j++ {
				// Calculate the time offset for this coefficient
				t := float64(j-r.filterHalf) - phaseOffset

				// Use lookup tables for better performance
				normalizedCutoff := float64(r.originalRate) / (2.0 * float64(r.targetRate))

				if t == 0 {
					r.polyphaseFilters[i][j] = 2.0 * normalizedCutoff
				} else {
					// Use the lookup tables instead of direct calculation
					sincArg := 2.0 * math.Pi * normalizedCutoff * t
					sincVal := r.lookupSinc(sincArg)
					windowVal := r.lookupWindow(float64(j) / float64(r.filterLen-1))
					r.polyphaseFilters[i][j] = sincVal * windowVal
				}
			}

			// Normalize the filter
			sum := 0.0
			for j := 0; j < r.filterLen; j++ {
				sum += r.polyphaseFilters[i][j]
			}

			// Avoid division by zero
			if sum != 0 {
				for j := 0; j < r.filterLen; j++ {
					r.polyphaseFilters[i][j] /= sum
				}
			}
		}
	} else {
		// Downsampling - only need one phase
		r.polyphaseFilters[0] = make([]float64, r.filterLen)
		copy(r.polyphaseFilters[0], r.filter)
	}
}

// precomputeTables creates lookup tables for sinc and window functions
func (r *StreamingResampler) precomputeTables() {
	// The tables size depends on the precision needed
	tableSize := 1024

	// Sinc table from -10π to 10π (typical range needed)
	r.sincTable = make([]float64, tableSize)
	for i := 0; i < tableSize; i++ {
		x := 20.0 * math.Pi * (float64(i)/float64(tableSize-1) - 0.5)
		if x == 0 {
			r.sincTable[i] = 1.0
		} else {
			r.sincTable[i] = math.Sin(x) / x
		}
	}

	// Window table from 0 to 1
	r.windowTable = make([]float64, tableSize)
	for i := 0; i < tableSize; i++ {
		x := float64(i) / float64(tableSize-1)
		r.windowTable[i] = kaiserValue(x, 14.0)
	}
}

// lookupSinc performs a fast table lookup for sinc values
func (r *StreamingResampler) lookupSinc(x float64) float64 {
	// Normalize x to the table range (-10π to 10π)
	x = math.Abs(x)
	if x > 10.0*math.Pi {
		return 0.0 // Assume zero beyond this range
	}

	// Map to table index
	idx := int(((x / (20.0 * math.Pi)) + 0.5) * float64(len(r.sincTable)-1))
	if idx < 0 {
		idx = 0
	} else if idx >= len(r.sincTable) {
		idx = len(r.sincTable) - 1
	}

	return r.sincTable[idx]
}

// lookupWindow performs a fast table lookup for window values
func (r *StreamingResampler) lookupWindow(x float64) float64 {
	if x < 0 {
		x = 0
	} else if x > 1 {
		x = 1
	}

	idx := int(x * float64(len(r.windowTable)-1))
	return r.windowTable[idx]
}

// Process resamples a chunk of audio with optimized algorithms
func (r *StreamingResampler) Process(chunk []float32) []float32 {
	if r.originalRate == r.targetRate {
		return chunk
	}

	// Create working buffer with history
	inputLen := len(chunk)
	workBuf := make([]float32, r.filterHalf+inputLen)

	// Copy history and current chunk
	copy(workBuf, r.history)
	copy(workBuf[r.filterHalf:], chunk)

	// Update history for next chunk
	if inputLen >= r.filterHalf {
		copy(r.history, chunk[inputLen-r.filterHalf:])
	} else {
		copy(r.history, r.history[inputLen:])
		copy(r.history[r.filterHalf-inputLen:], chunk)
	}

	// Calculate output size
	outputLen := int(math.Ceil(float64(inputLen) * r.ratio))
	if r.scratchBuffer == nil || len(r.scratchBuffer) < outputLen {
		r.scratchBuffer = make([]float32, outputLen)
	}
	output := r.scratchBuffer[:outputLen]

	// Choose appropriate resampling method
	switch {
	case r.isIntegerRatio && r.originalRate < r.targetRate:
		// Integer upsampling ratio (like 16kHz -> 48kHz)
		r.processIntegerUpsampling(workBuf, output)
	case r.originalRate == 32000 && r.targetRate == 48000:
		// Special case for 32kHz -> 48kHz (3:2 ratio)
		r.process32kTo48k(workBuf, output)
	case r.isIntegerRatio && r.originalRate > r.targetRate:
		// Integer downsampling ratio
		r.processIntegerDownsampling(workBuf, output)
	default:
		// General case for arbitrary ratio
		r.processArbitraryRatio(workBuf, output)
	}

	// Return a copy to avoid shared buffer issues
	result := make([]float32, outputLen)
	copy(result, output)
	return result
}

// processIntegerUpsampling is optimized for integer upsampling ratios like 16kHz -> 48kHz (3x)
func (r *StreamingResampler) processIntegerUpsampling(workBuf, output []float32) {
	ratio := r.integerRatio
	outputLen := len(output)

	if r.numPhases == ratio {
		// Use precomputed polyphase filters
		for i := 0; i < outputLen; i++ {
			// Calculate input sample and phase
			inputIdx := i / ratio
			phase := i % ratio

			// Apply the phase-specific filter
			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.polyphaseFilters[phase][j]
				}
			}

			output[i] = float32(sum)
		}
	} else {
		// Fallback to general filter
		for i := 0; i < outputLen; i++ {
			inputIdx := i / ratio

			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.filter[j]
				}
			}

			output[i] = float32(sum)
		}
	}
}

// processIntegerDownsampling is optimized for integer downsampling ratios
func (r *StreamingResampler) processIntegerDownsampling(workBuf, output []float32) {
	ratio := r.integerRatio
	outputLen := len(output)

	// Integer downsampling is straightforward - just take every Nth sample
	// after filtering
	for i := 0; i < outputLen; i++ {
		inputIdx := i * ratio

		var sum float64
		for j := 0; j < r.filterLen; j++ {
			idx := inputIdx - r.filterHalf + j + r.filterHalf

			if idx >= 0 && idx < len(workBuf) {
				sum += float64(workBuf[idx]) * r.filter[j]
			}
		}

		output[i] = float32(sum)
	}
}

// process32kTo48k is specially optimized for 32kHz -> 48kHz conversion (3:2 ratio)
func (r *StreamingResampler) process32kTo48k(workBuf, output []float32) {
	outputLen := len(output)

	if r.numPhases == 3 {
		// Use 3-phase polyphase filters for 3:2 ratio
		for i := 0; i < outputLen; i++ {
			// For 3:2 ratio, map output index to input position and phase
			// This pattern repeats: i=0->0,0; i=1->0,1; i=2->0,2; i=3->2,0; i=4->2,1; i=5->2,2...
			inputPos := (i * 2) / 3
			phase := i % 3

			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputPos - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.polyphaseFilters[phase][j]
				}
			}

			output[i] = float32(sum)
		}
	} else {
		// Fallback to general method for 3:2 ratio
		for i := 0; i < outputLen; i++ {
			inputPos := float64(i) * 2.0 / 3.0
			inputIdx := int(inputPos)

			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.filter[j]
				}
			}

			output[i] = float32(sum)
		}
	}
}

// processArbitraryRatio handles general case resampling with arbitrary ratio
func (r *StreamingResampler) processArbitraryRatio(workBuf, output []float32) {
	outputLen := len(output)

	// For arbitrary ratios, we can generate the filter dynamically for each output sample
	// using our precomputed lookup tables for better performance
	for i := 0; i < outputLen; i++ {
		// Convert output sample index to input sample position
		outputTime := float64(i) / float64(r.targetRate)
		inputSamplePos := outputTime * float64(r.originalRate)
		inputSampleIdx := int(inputSamplePos)

		// Calculate the fractional part for filter phase
		frac := inputSamplePos - float64(inputSampleIdx)

		// Either use precomputed filter or generate one on-the-fly using lookup tables
		if frac < 0.001 && r.polyphaseFilters != nil && len(r.polyphaseFilters) > 0 {
			// If we're close to an integer sample, use precomputed filter
			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputSampleIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.filter[j]
				}
			}
			output[i] = float32(sum)
		} else {
			// Generate filter for this specific fractional offset using lookup tables
			var sum float64
			normalizedCutoff := float64(math.Min(float64(r.originalRate), float64(r.targetRate))) /
				(2.0 * float64(r.targetRate))

			for j := 0; j < r.filterLen; j++ {
				// Calculate time offset with fractional correction
				t := float64(j-r.filterHalf) - frac

				// Calculate sinc and window using lookup tables
				var filterVal float64
				if t == 0 {
					filterVal = 2.0 * normalizedCutoff
				} else {
					// Use lookup tables for sinc and window
					sincVal := r.lookupSinc(2.0 * math.Pi * normalizedCutoff * t)
					windowVal := r.lookupWindow(float64(j) / float64(r.filterLen-1))
					filterVal = sincVal * windowVal
				}

				// Apply filter
				idx := inputSampleIdx - r.filterHalf + j + r.filterHalf
				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * filterVal
				}
			}

			// Normalize on-the-fly filters
			if sum != 0 {
				output[i] = float32(sum)
			} else {
				// Fallback to precomputed filter if we have issues
				sum = 0
				for j := 0; j < r.filterLen; j++ {
					idx := inputSampleIdx - r.filterHalf + j + r.filterHalf

					if idx >= 0 && idx < len(workBuf) {
						sum += float64(workBuf[idx]) * r.filter[j]
					}
				}
				output[i] = float32(sum)
			}
		}
	}
}

// ProcessBirdNETChunk is optimized for the BirdNET application
func (r *StreamingResampler) ProcessBirdNETChunk(chunk []float32) []float32 {
	// Special optimization paths for common BirdNET sample rates
	switch {
	case r.originalRate == 16000 && r.targetRate == 48000:
		// Special optimization for 16kHz -> 48kHz (common case for BirdNET)
		return r.Process16kTo48k(chunk)
	case r.originalRate == 32000 && r.targetRate == 48000:
		// Special optimization for 32kHz -> 48kHz (another common case)
		return r.Process(chunk) // Using optimized 32k->48k path
	default:
		// For other rates, use general process
		return r.Process(chunk)
	}
}

// Process16kTo48k is a highly optimized function specific to 16kHz -> 48kHz conversion
func (r *StreamingResampler) Process16kTo48k(chunk []float32) []float32 {
	// Special case for 16kHz -> 48kHz (3x upsampling)
	inputLen := len(chunk)
	outputLen := inputLen * 3

	// Create working buffer with history
	workBuf := make([]float32, r.filterHalf+inputLen)
	copy(workBuf, r.history)
	copy(workBuf[r.filterHalf:], chunk)

	// Update history for next chunk
	if inputLen >= r.filterHalf {
		copy(r.history, chunk[inputLen-r.filterHalf:])
	} else {
		copy(r.history, r.history[inputLen:])
		copy(r.history[r.filterHalf-inputLen:], chunk)
	}

	// Allocate output
	result := make([]float32, outputLen)

	// Use polyphase filters if available
	if r.numPhases == 3 {
		// Process each output sample using phase-specific filter
		for i := 0; i < outputLen; i++ {
			inputIdx := i / 3
			phase := i % 3

			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					// Use precomputed phase filter
					sum += float64(workBuf[idx]) * r.polyphaseFilters[phase][j]
				}
			}

			result[i] = float32(sum)
		}
	} else {
		// Fallback if polyphase filters weren't created
		for i := 0; i < outputLen; i++ {
			inputIdx := i / 3

			var sum float64
			for j := 0; j < r.filterLen; j++ {
				idx := inputIdx - r.filterHalf + j + r.filterHalf

				if idx >= 0 && idx < len(workBuf) {
					sum += float64(workBuf[idx]) * r.filter[j]
				}
			}

			result[i] = float32(sum)
		}
	}

	return result
}

// Reset resets the resampler state
func (r *StreamingResampler) Reset() {
	// Clear history buffer
	for i := range r.history {
		r.history[i] = 0
	}
}

// kaiserValue computes Kaiser window value directly (for precomputation)
func kaiserValue(x, beta float64) float64 {
	// Map x from [0,1] to [-1,1]
	x = 2*x - 1

	// Calculate window value
	if x < -1 || x > 1 {
		return 0
	}

	arg := beta * math.Sqrt(1.0-x*x)
	return bessel0(arg) / bessel0(beta)
}

// designLowpassFilter creates a windowed-sinc lowpass filter
func designLowpassFilter(cutoff float64, length int) []float64 {
	filter := make([]float64, length)
	window := make([]float64, length)
	sinc := make([]float64, length)

	// Generate Kaiser window
	kaiserWindow(window, 14.0)

	// Generate sinc filter
	generateSincFilter(sinc, cutoff)

	// Apply window to sinc filter
	for i := 0; i < length; i++ {
		filter[i] = sinc[i] * window[i]
	}

	// Normalize for unity gain at DC
	sum := 0.0
	for i := 0; i < length; i++ {
		sum += filter[i]
	}
	for i := 0; i < length; i++ {
		filter[i] /= sum
	}

	return filter
}

// kaiserWindow generates a Kaiser window
func kaiserWindow(window []float64, beta float64) {
	length := len(window)

	denominator := bessel0(beta)
	for i := 0; i < length; i++ {
		// Calculate Kaiser window value
		x := 2.0*float64(i)/float64(length-1) - 1.0
		window[i] = bessel0(beta*math.Sqrt(1.0-x*x)) / denominator
	}
}

// bessel0 approximates the modified Bessel function of the first kind, order 0
func bessel0(x float64) float64 {
	// Use series expansion for I₀(x)
	sum := 1.0
	factorial := 1.0
	halfx := x / 2.0
	pow := 1.0

	// Typically 12-15 terms are sufficient for audio processing accuracy
	for k := 1; k < 15; k++ {
		pow *= halfx * halfx
		factorial *= float64(k) * float64(k)
		term := pow / factorial
		sum += term

		// Early termination if we've reached sufficient precision
		if term < 1e-12 {
			break
		}
	}

	return sum
}

// generateSincFilter creates a sinc filter with the given cutoff frequency
func generateSincFilter(filter []float64, cutoff float64) {
	length := len(filter)
	halfLen := (length - 1) / 2

	for i := 0; i < length; i++ {
		if i == halfLen {
			// Handle center point separately to avoid division by zero
			filter[i] = 2.0 * cutoff
		} else {
			// Calculate normalized time
			t := float64(i - halfLen)

			// Apply sinc function: sin(2πfct) / πt where fc is the cutoff frequency
			filter[i] = math.Sin(2.0*math.Pi*cutoff*t) / (math.Pi * t)
		}
	}
}

// Example of how to use the optimized streaming resampler in BirdNET-Go

// ExampleBirdNETUsage demonstrates how to use the resampler for BirdNET processing
func ExampleBirdNETUsage() {
	// Create the optimized resampler once at initialization
	resampler := NewStreamingResampler(16000, 48000) // Convert from 16kHz to 48kHz

	// Process chunks as they come in
	for {
		// Read a chunk from your WAV or FLAC file
		chunk := readNextChunk() // This would be your function to read audio data
		if chunk == nil {
			break
		}

		// Process with the optimized path for BirdNET
		resampledChunk := resampler.ProcessBirdNETChunk(chunk)

		// Feed resampled audio to BirdNET
		processBirdNET(resampledChunk) // This would be your function to analyze audio
	}
}

// ExampleDirectUsage shows how to use the resampler directly
func ExampleDirectUsage() {
	// For standalone usage with byte slices containing float32 PCM data
	originalSampleRate := 44100
	targetSampleRate := 48000

	// Create resampler
	resampler := NewStreamingResampler(originalSampleRate, targetSampleRate)

	// Process audio data (assuming float32 PCM data in the range of -1.0 to 1.0)
	inputData := []float32{0.1, 0.2, -0.1, -0.2} // Your audio samples
	resampledData := resampler.Process(inputData)

	// Use the resampled data
	fmt.Printf("Resampled %d samples to %d samples\n", len(inputData), len(resampledData))
}

// ExampleFileResampling shows how to use the resampler with file processing
func ExampleFileResampling() {
	// Create a file manager with automatic resampling to 48kHz (enabled by default)
	fileManager := NewManager(false)

	// Process an audio file - all audio will be automatically resampled to 48kHz if needed
	processor := func(chunk Chunk) error {
		// The chunk.Data will already be resampled to 48kHz if the original file wasn't 48kHz
		fmt.Printf("Processing %d samples at 48kHz\n", len(chunk.Data))
		return nil
	}

	ctx := context.Background()
	err := fileManager.ProcessAudioFile(ctx, "audio_file.wav", 3.0, 1.5, processor)
	if err != nil {
		fmt.Printf("Error processing file: %v\n", err)
	}
}

// ExampleCustomSampleRate shows how to use the resampler with a custom target sample rate
func ExampleCustomSampleRate() {
	// Create a file manager with custom resampling options
	// Parameters: debug, targetSampleRate, enableResampling
	fileManager := NewManagerWithOptions(false, 44100, true)

	// Process an audio file - it will be automatically resampled to 44.1kHz if needed
	processor := func(chunk Chunk) error {
		// The chunk.Data will be at 44.1kHz
		fmt.Printf("Processing %d samples at 44.1kHz\n", len(chunk.Data))
		return nil
	}

	ctx := context.Background()
	err := fileManager.ProcessAudioFile(ctx, "audio_file.wav", 3.0, 1.5, processor)
	if err != nil {
		fmt.Printf("Error processing file: %v\n", err)
	}
}

// These are example functions that would be implemented by the user
func readNextChunk() []float32 {
	// In a real application, this would read from a file or stream
	return nil
}

func processBirdNET(audio []float32) {
	// In a real application, this would send audio to BirdNET analyzer
}
