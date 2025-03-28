package analysis

import (
	"log"
	"sync"
	"time"

	"github.com/tphakala/birdnet-go/internal/audio/buffer"
	"github.com/tphakala/birdnet-go/internal/audio/model"
	"github.com/tphakala/birdnet-go/internal/conf"
)

// BufferAnalyzer handles the periodic polling of audio buffers and analysis
type BufferAnalyzer struct {
	sourceID         string
	bufferManager    buffer.BufferManagerInterface
	modelManager     *model.Manager
	settings         *conf.Settings
	pollingInterval  time.Duration
	effectiveAdvance time.Duration
	lastAnalysisTime time.Time
	ticker           *time.Ticker
	stopChan         chan struct{}
	wg               *sync.WaitGroup
	tickCounter      int
}

// NewBufferAnalyzer creates a new buffer analyzer for a specific audio source
func NewBufferAnalyzer(
	sourceID string,
	bufMgr buffer.BufferManagerInterface,
	modelMgr *model.Manager,
	settings *conf.Settings,
	wg *sync.WaitGroup,
) *BufferAnalyzer {
	// Fixed polling interval for consistent checking
	const pollingInterval = 100 * time.Millisecond

	// Calculate how much buffer needs to advance between analyses
	// based on BirdNET window (3 seconds) and overlap setting
	windowDuration := 3 * time.Second
	overlapDuration := time.Duration(settings.BirdNET.Overlap * float64(time.Second))
	effectiveAdvance := windowDuration - overlapDuration

	return &BufferAnalyzer{
		sourceID:         sourceID,
		bufferManager:    bufMgr,
		modelManager:     modelMgr,
		settings:         settings,
		pollingInterval:  pollingInterval,
		effectiveAdvance: effectiveAdvance,
		lastAnalysisTime: time.Now().Add(-effectiveAdvance), // Start ready to analyze
		stopChan:         make(chan struct{}),
		wg:               wg,
	}
}

// Start begins the polling and analysis process
func (ba *BufferAnalyzer) Start() {
	// Debug: Check the actual buffer capacity if it exists
	if ba.bufferManager.HasAnalysisBuffer(ba.sourceID) {
		if capacity, err := ba.bufferManager.GetAnalysisBufferCapacity(ba.sourceID); err == nil {
			// Calculate how many seconds this buffer can hold
			sampleRate := conf.SampleRate
			bytesPerSample := conf.BitDepth / 8
			numChannels := conf.NumChannels
			bytesPerSecond := sampleRate * bytesPerSample * numChannels

			bufferSeconds := float64(capacity) / float64(bytesPerSecond)
			log.Printf("🔍 Analysis buffer for %s has capacity of %d bytes (%.2f seconds at %d Hz)",
				ba.sourceID, capacity, bufferSeconds, sampleRate)
		}
	}

	log.Printf("📊 Starting polling analyzer for source %s with fixed interval %v, advance %v",
		ba.sourceID, ba.pollingInterval, ba.effectiveAdvance)

	// Create the ticker with the configured interval
	ba.ticker = time.NewTicker(ba.pollingInterval)

	ba.wg.Add(1)
	go ba.run()
}

// Stop halts the polling and analysis process
func (ba *BufferAnalyzer) Stop() {
	close(ba.stopChan)
}

// run is the main analyzer loop
func (ba *BufferAnalyzer) run() {
	defer ba.wg.Done()
	defer ba.ticker.Stop()

	for {
		select {
		case <-ba.stopChan:
			log.Printf("🛑 Stopping polling analyzer for source %s", ba.sourceID)
			return

		case currentTime := <-ba.ticker.C:
			ba.tickCounter++

			// Skip this tick if we're not due for an analysis yet based on rate limiting
			if currentTime.Sub(ba.lastAnalysisTime) < ba.effectiveAdvance {
				continue
			}

			// Check for and analyze data
			ba.checkAndAnalyze(currentTime)
		}
	}
}

// checkAndAnalyze checks if there is data available and analyzes it
func (ba *BufferAnalyzer) checkAndAnalyze(currentTime time.Time) {
	// Skip if no analysis buffer exists for this source
	if !ba.bufferManager.HasAnalysisBuffer(ba.sourceID) {
		if ba.tickCounter%100 == 0 { // Log less frequently
			log.Printf("⏳ No analysis buffer for %s (tick %d)", ba.sourceID, ba.tickCounter)
		}
		return
	}

	// Use buffer configuration
	bufferConfig := buffer.NewDefaultBufferConfig()

	// Try to read from the analysis buffer - this will return nil if not enough data
	analysisData, err := ba.bufferManager.ReadFromAnalysisBuffer(ba.sourceID, bufferConfig)

	log.Printf("🔍 Read %d bytes from %s analysis buffer (tick %d)",
		len(analysisData), ba.sourceID, ba.tickCounter)

	// Handle errors
	if err != nil {
		if ba.tickCounter%100 == 0 { // Log less frequently
			log.Printf("⚠️ Error reading from buffer for %s: %v (tick %d)",
				ba.sourceID, err, ba.tickCounter)
		}
		return
	}

	// Skip if not enough data yet
	if analysisData == nil {
		if ba.tickCounter%100 == 0 { // Log less frequently
			log.Printf("⏳ Buffer for %s not ready for analysis yet (tick %d)",
				ba.sourceID, ba.tickCounter)
		}
		return
	}

	// We have data and we're ready for analysis - update last analysis time
	ba.lastAnalysisTime = currentTime

	// If debug log data length
	if ba.settings.Debug {
		log.Printf("✅ Read %d bytes from %s analysis buffer (tick %d)",
			len(analysisData), ba.sourceID, ba.tickCounter)
	}

	// Analyze the audio data
	if err := ba.modelManager.Analyze(ba.sourceID, analysisData, currentTime.Unix()); err != nil {
		log.Printf("❌ Error analyzing audio data from %s: %v", ba.sourceID, err)
	} else if ba.tickCounter%10 == 0 || ba.tickCounter < 10 {
		// Log success (not too often)
		log.Printf("🔊 Analyzed audio data from %s (tick %d)", ba.sourceID, ba.tickCounter)
	}
}

// StartPollingAnalyzer is a convenience function to create and start a buffer analyzer
func StartPollingAnalyzer(
	wg *sync.WaitGroup,
	settings *conf.Settings,
	quitChan chan struct{},
	bufMgr buffer.BufferManagerInterface,
	modelMgr *model.Manager,
	sourceID string,
) {
	analyzer := NewBufferAnalyzer(sourceID, bufMgr, modelMgr, settings, wg)
	analyzer.Start()

	// Setup cleanup when quit channel is signaled
	go func() {
		<-quitChan
		analyzer.Stop()
	}()
}
