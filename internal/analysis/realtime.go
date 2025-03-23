package analysis

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"runtime"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/tphakala/birdnet-go/internal/analysis/processor"
	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/birdnet-go/internal/audio/buffer"
	"github.com/tphakala/birdnet-go/internal/audio/capture"
	"github.com/tphakala/birdnet-go/internal/audio/model"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
	"github.com/tphakala/birdnet-go/internal/diskmanager"
	"github.com/tphakala/birdnet-go/internal/httpcontroller"
	"github.com/tphakala/birdnet-go/internal/httpcontroller/handlers"
	"github.com/tphakala/birdnet-go/internal/imageprovider"
	"github.com/tphakala/birdnet-go/internal/telemetry"
	"github.com/tphakala/birdnet-go/internal/weather"
)

// audioLevelChan is a channel to send audio level updates
var audioLevelChan = make(chan audio.AudioLevelData, 100)

// Global model manager instance
var modelManager *model.Manager

// Global buffer manager for handling audio data
var bufferManager buffer.BufferManagerInterface

// RealtimeAnalysis initiates the BirdNET Analyzer in real-time mode and waits for a termination signal.
func RealtimeAnalysis(settings *conf.Settings, notificationChan chan handlers.Notification) error {
	// Initialize BirdNET interpreter
	if err := initializeBirdNET(settings); err != nil {
		return fmt.Errorf("failed to initialize BirdNET: %w", err)
	}

	// Initialize model manager - still global but will be injected into functions that need it
	modelManager = model.NewManager(settings)
	modelManager.SetDefaultInstance(bn)

	// Initialize buffer manager - still global but will be injected into functions that need it
	bufferFactory := buffer.NewBufferFactory()
	bufferManager = bufferFactory.CreateBufferManager()

	// Log debug status
	if settings.BirdNET.Debug {
		log.Println("🔍 BirdNET Debug mode enabled - showing detailed prediction results")
	}

	// Get system details with golps
	info, err := host.Info()
	if err != nil {
		fmt.Printf("❌ Error retrieving host info: %v\n", err)
	}

	var hwModel string
	// Print SBC hardware details
	if conf.IsLinuxArm64() {
		hwModel = conf.GetBoardModel()
		// remove possible new line from hwModel
		hwModel = strings.TrimSpace(hwModel)
	} else {
		hwModel = "unknown"
	}

	// Print platform, OS etc. details
	fmt.Printf("System details: %s %s %s on %s hardware\n", info.OS, info.Platform, info.PlatformVersion, hwModel)

	// Log the start of BirdNET-Go Analyzer in realtime mode and its configurations.
	fmt.Printf("Starting analyzer in realtime mode. Threshold: %v, overlap: %v, sensitivity: %v, interval: %v\n",
		settings.BirdNET.Threshold,
		settings.BirdNET.Overlap,
		settings.BirdNET.Sensitivity,
		settings.Realtime.Interval)

	// Initialize database access.
	dataStore := datastore.New(settings)

	// Open a connection to the database and handle possible errors.
	if err := dataStore.Open(); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Ensure the database connection is closed when the function returns.
	defer closeDataStore(dataStore)

	// Initialize the control channel for restart control.
	controlChan := make(chan string, 1)
	// Initialize the restart channel for capture restart control.
	restartChan := make(chan struct{}, 3)
	// quitChannel is used to signal the goroutines to stop.
	quitChan := make(chan struct{})

	// Initialize audioLevelChan, used to visualize audio levels on web ui
	audioLevelChan = make(chan audio.AudioLevelData, 100)

	// Check if we have any audio sources configured
	if len(settings.Realtime.RTSP.URLs) == 0 && settings.Realtime.Audio.Source == "" {
		log.Println("⚠️  Starting without active audio sources. You can configure audio devices or RTSP streams through the web interface.")
	}

	// Queue is now initialized at package level in birdnet package
	// Optionally resize the queue if needed
	birdnet.ResizeQueue(5)

	// Initialize Prometheus metrics manager
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		return fmt.Errorf("error initializing metrics: %w", err)
	}

	var birdImageCache *imageprovider.BirdImageCache
	if settings.Realtime.Dashboard.Thumbnails.Summary || settings.Realtime.Dashboard.Thumbnails.Recent {
		// Initialize the bird image cache
		birdImageCache = initBirdImageCache(dataStore, metrics)
	} else {
		birdImageCache = nil
	}

	// Initialize processor
	proc := processor.New(settings, dataStore, bn, metrics, birdImageCache, bufferManager)

	// Initialize and start the HTTP server
	httpServer := httpcontroller.New(settings, dataStore, birdImageCache, audioLevelChan, controlChan, proc)
	httpServer.Start()

	// Initialize the wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start audio capture with new audio components
	streamManager, ffmpegMonitor := startAudioCapture(&wg, settings, quitChan, restartChan, audioLevelChan)

	// start cleanup of clips
	if conf.Setting().Realtime.Audio.Export.Retention.Policy != "none" {
		startClipCleanupMonitor(&wg, quitChan, dataStore)
	}

	// start weather polling
	if settings.Realtime.Weather.Provider != "none" {
		startWeatherPolling(&wg, settings, dataStore, quitChan)
	}

	// start telemetry endpoint
	startTelemetryEndpoint(&wg, settings, metrics, quitChan)

	// start control monitor for hot reloads
	startControlMonitor(&wg, controlChan, quitChan, restartChan, notificationChan, proc, streamManager, ffmpegMonitor)

	// start quit signal monitor
	monitorCtrlC(quitChan)

	// loop to monitor quit and restart channels
	for {
		select {
		case <-quitChan:
			// Clean up all buffers
			bufferManager.CleanupAllBuffers()

			// Close controlChan to signal that no restart attempts should be made.
			close(controlChan)
			// Wait for all goroutines to finish.
			wg.Wait()
			// Delete the BirdNET interpreter.
			bn.Delete()
			// Return nil to indicate that the program exited successfully.
			return nil

		case <-restartChan:
			// Handle the restart signal.
			fmt.Println("🔄 Restarting audio capture")
			streamManager, ffmpegMonitor = startAudioCapture(&wg, settings, quitChan, restartChan, audioLevelChan)
			// Update the control monitor with new components
			startControlMonitor(&wg, controlChan, quitChan, restartChan, notificationChan, proc, streamManager, ffmpegMonitor)
		}
	}
}

// getFFmpegPath returns the appropriate FFmpeg path based on settings and OS
func getFFmpegPath(settings *conf.Settings) string {
	ffmpegPath := settings.Realtime.Audio.FfmpegPath
	if ffmpegPath != "" {
		return ffmpegPath
	}

	// Default ffmpeg path if not specified
	if runtime.GOOS == "windows" {
		ffmpegPath = "ffmpeg.exe"
	} else {
		ffmpegPath = "/usr/bin/ffmpeg"
	}
	log.Printf("⚠️ FFmpeg path not specified, using default: %s", ffmpegPath)
	return ffmpegPath
}

// createBuffersIfNeeded ensures both analysis and capture buffers exist for a source
func createBuffersIfNeeded(bufMgr buffer.BufferManagerInterface, sourceID string) error {
	// Create analysis buffer if needed
	if !bufMgr.HasAnalysisBuffer(sourceID) {
		// Calculate buffer size - 9 seconds buffer (3x the 3 second BirdNET window)
		sampleRate := conf.SampleRate
		bytesPerSample := conf.BitDepth / 8
		numChannels := conf.NumChannels
		bufferSize := 9 * sampleRate * bytesPerSample * numChannels

		if err := bufMgr.AllocateAnalysisBuffer(bufferSize, sourceID); err != nil {
			return fmt.Errorf("failed to create analysis buffer: %w", err)
		}
		log.Printf("✅ Created analysis buffer for source: %s", sourceID)
	}

	// Create capture buffer if audio export is enabled
	if conf.Setting().Realtime.Audio.Export.Enabled && !bufMgr.HasCaptureBuffer(sourceID) {
		captureDuration := 60 // 60 seconds by default
		if err := bufMgr.AllocateCaptureBuffer(
			captureDuration, conf.SampleRate, conf.BitDepth/8, sourceID,
		); err != nil {
			// Non-fatal error - we can continue without capture buffer
			log.Printf("⚠️ Failed to create capture buffer for %s: %v", sourceID, err)
		} else {
			log.Printf("✅ Created capture buffer for source: '%s' (duration: %ds)", sourceID, captureDuration)
		}
	}

	return nil
}

// writeToBuffers writes audio data to both analysis and capture buffers
func writeToBuffers(bufMgr buffer.BufferManagerInterface, sourceID string, data []byte) error {
	// Write data to the analysis buffer
	if err := bufMgr.WriteToAnalysisBuffer(sourceID, data); err != nil {
		return fmt.Errorf("error writing to analysis buffer: %w", err)
	}

	// Write data to the capture buffer if it exists
	if bufMgr.HasCaptureBuffer(sourceID) {
		if err := bufMgr.WriteToCaptureBuffer(sourceID, data); err != nil {
			// Non-fatal error, we can continue analysis without successful capture
			log.Printf("⚠️ Error writing to capture buffer for %s: %v", sourceID, err)
		}
	}

	return nil
}

// processAudioData handles the common audio processing logic for all audio sources
func processAudioData(sourceID string, data []byte, startTime time.Time,
	bufMgr buffer.BufferManagerInterface, modelMgr *model.Manager) {
	// Early return if model manager is not initialized
	if modelMgr == nil {
		log.Printf("⚠️ Model manager not initialized, cannot process audio data from %s", sourceID)
		return
	}

	// Create both buffers if needed
	if err := createBuffersIfNeeded(bufMgr, sourceID); err != nil {
		log.Printf("❌ %v", err)
		return
	}

	// Write data to both buffers
	if err := writeToBuffers(bufMgr, sourceID, data); err != nil {
		log.Printf("❌ %v", err)
		return
	}

	// Use a proper buffer config with default settings
	bufferConfig := buffer.NewDefaultBufferConfig()

	// Read from the analysis buffer using the proper buffer API
	analysisData, err := bufMgr.ReadFromAnalysisBuffer(sourceID, bufferConfig)
	if err != nil || analysisData == nil {
		// Not enough data yet or error, just return without analyzing
		return
	}

	// Use the manager to analyze audio data
	if err := modelMgr.Analyze(sourceID, analysisData, startTime.Unix()); err != nil {
		log.Printf("❌ Error analyzing audio data from %s: %v", sourceID, err)
	}
}

// removeBufferIfExists removes a buffer for the given source if it exists
func removeBufferIfExists(bufferManager buffer.BufferManagerInterface, bufferType, sourceID string) {
	if bufferType == "analysis" {
		if bufferManager.HasAnalysisBuffer(sourceID) {
			if err := bufferManager.RemoveAnalysisBuffer(sourceID); err != nil {
				log.Printf("⚠️ Error removing analysis buffer for %s: %v", sourceID, err)
			} else {
				log.Printf("✅ Removed analysis buffer for %s", sourceID)
			}
		}
	} else if bufferType == "capture" {
		if bufferManager.HasCaptureBuffer(sourceID) {
			if err := bufferManager.RemoveCaptureBuffer(sourceID); err != nil {
				log.Printf("⚠️ Error removing capture buffer for %s: %v", sourceID, err)
			} else {
				log.Printf("✅ Removed capture buffer for %s", sourceID)
			}
		}
	}
}

// RemoveAllBuffersForSource removes both analysis and capture buffers for a given source
func RemoveAllBuffersForSource(bufMgr buffer.BufferManagerInterface, sourceID string) {
	// Remove analysis buffer if it exists
	if bufMgr.HasAnalysisBuffer(sourceID) {
		if err := bufMgr.RemoveAnalysisBuffer(sourceID); err != nil {
			log.Printf("⚠️ Error removing analysis buffer for %s: %v", sourceID, err)
		} else {
			log.Printf("✅ Removed analysis buffer for %s", sourceID)
		}
	}

	// Remove capture buffer if it exists
	if bufMgr.HasCaptureBuffer(sourceID) {
		if err := bufMgr.RemoveCaptureBuffer(sourceID); err != nil {
			log.Printf("⚠️ Error removing capture buffer for %s: %v", sourceID, err)
		} else {
			log.Printf("✅ Removed capture buffer for %s", sourceID)
		}
	}
}

// initAudioDevice initializes and starts the audio device capture
func initAudioDevice(wg *sync.WaitGroup, settings *conf.Settings, quitChan chan struct{},
	bufMgr buffer.BufferManagerInterface, modelMgr *model.Manager) {

	deviceName := settings.Realtime.Audio.Source
	if deviceName == "" {
		// If no device is configured, check if we need to clean up previous device buffers
		if bufMgr != nil {
			// Clean up any buffers from previous audio device configuration
			RemoveAllBuffersForSource(bufMgr, "malgo")
		}
		return
	}

	log.Printf("🎤 Initializing audio device: %s", deviceName)

	// Create audio context factory and context
	factory := capture.NewContextFactory(false)
	audioCtx, err := factory.CreateAudioContext(func(msg string) {
		//log.Printf("Audio: %s", msg)
	})
	if err != nil {
		log.Printf("❌ Failed to create audio context: %v", err)
		return
	}

	// Create a device manager to handle audio card capture
	captureManager := capture.NewDeviceManager(audioCtx, nil)

	// Set data callback to process audio from the device
	captureManager.SetDataCallback(func(sourceID string, data []byte, frameCount uint32) {
		// Use our consolidated processing function
		processAudioData(sourceID, data, time.Now(), bufMgr, modelMgr)
	})

	// Start capturing from the audio device
	if err := captureManager.StartCapture(deviceName, conf.SampleRate, conf.NumChannels); err != nil {
		log.Printf("❌ Error starting capture from audio device %s: %v", deviceName, err)
		// Clean up the context if we fail to start capture
		if uninitErr := audioCtx.Uninit(); uninitErr != nil {
			log.Printf("⚠️ Error uninitializing audio context: %v", uninitErr)
		}
		return
	}

	log.Printf("✅ Started audio capture from device: %s", deviceName)

	// Add cleanup for the device manager
	registerCleanupHandler(wg, quitChan, func() {
		log.Println("Stopping audio device capture...")
		captureManager.Close()
		if err := audioCtx.Uninit(); err != nil {
			log.Printf("⚠️ Error uninitializing audio context during cleanup: %v", err)
		}
	})
}

// registerCleanupHandler adds a cleanup handler that runs when quit signal is received
func registerCleanupHandler(wg *sync.WaitGroup, quitChan chan struct{}, cleanupFunc func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-quitChan
		cleanupFunc()
	}()
}

// initRTSPStreams initializes the RTSP streams specified in the settings
func initRTSPStreams(settings *conf.Settings, streamManager audio.StreamManager) {
	if len(settings.Realtime.RTSP.URLs) == 0 {
		return
	}

	for _, url := range settings.Realtime.RTSP.URLs {
		if url == "" {
			continue
		}

		transport := settings.Realtime.RTSP.Transport
		if transport == "" {
			transport = "tcp" // Default to TCP if not specified
		}

		if err := streamManager.StartStream(url, transport); err != nil {
			log.Printf("❌ Error starting stream %s: %v", url, err)
		} else {
			log.Printf("✅ Started stream: %s", url)
		}
	}
}

// initStreamManager initializes and configures the stream manager
func initStreamManager(ffmpegPath string, audioLevelChan chan audio.AudioLevelData,
	bufMgr buffer.BufferManagerInterface, modelMgr *model.Manager) (audio.StreamManager, audio.FFmpegMonitorInterface, error) {

	// Create stream manager and FFmpeg monitor using the provided factory function
	streamManager, ffmpegMonitor := audio.CreateAudioComponents(ffmpegPath)
	if streamManager == nil || ffmpegMonitor == nil {
		return nil, nil, fmt.Errorf("failed to create audio components with FFmpeg path: %s", ffmpegPath)
	}

	// Create a data callback closure that handles the audio processing
	dataCallback := func(sourceID, sourceName string, data []byte) {
		// Skip processing if model manager is not available
		if modelMgr == nil {
			log.Printf("⚠️ Model manager not initialized, cannot process audio data from %s", sourceID)
			return
		}

		// Create both buffers if needed
		if err := createBuffersIfNeeded(bufMgr, sourceID); err != nil {
			log.Printf("❌ Failed to create buffers for %s: %v", sourceID, err)
			return
		}

		// Write to both buffers - analysis and capture if it exists
		if err := writeToBuffers(bufMgr, sourceID, data); err != nil {
			log.Printf("❌ %v", err)
			return
		}

		// Get analysis data using default buffer config
		bufferConfig := buffer.NewDefaultBufferConfig()
		analysisData, err := bufMgr.ReadFromAnalysisBuffer(sourceID, bufferConfig)
		if err != nil || analysisData == nil {
			// Not enough data yet or error, just return without analyzing
			return
		}

		// Analyze the audio data
		now := time.Now()
		if err := modelMgr.Analyze(sourceID, analysisData, now.Unix()); err != nil {
			log.Printf("❌ Error analyzing audio data from %s: %v", sourceID, err)
		}
	}

	// Set up callbacks for the stream manager
	streamManager.SetCallbacks(
		// Data callback - process the audio data
		dataCallback,
		// Level callback - forward audio levels to the UI
		func(levelData audio.AudioLevelData) {
			select {
			case audioLevelChan <- levelData:
				// Audio level sent to channel
			default:
				// Channel full, skip this update
			}
		},
		// Restart callback
		func() {
			log.Printf("🔄 Stream restart detected")
		},
	)

	// Start the stream manager
	if err := streamManager.Start(); err != nil {
		return streamManager, ffmpegMonitor, fmt.Errorf("failed to start stream manager: %w", err)
	}

	log.Printf("✅ Initialized stream manager with FFmpeg path: %s", ffmpegPath)
	return streamManager, ffmpegMonitor, nil
}

// startAudioCapture initializes and starts the audio capture routine in a new goroutine.
func startAudioCapture(wg *sync.WaitGroup, settings *conf.Settings, quitChan, restartChan chan struct{},
	audioLevelChan chan audio.AudioLevelData) (audio.StreamManager, audio.FFmpegMonitorInterface) {

	// Get the appropriate FFmpeg path
	ffmpegPath := getFFmpegPath(settings)

	// Initialize stream manager with proper buffer and model managers
	streamManager, ffmpegMonitor, err := initStreamManager(ffmpegPath, audioLevelChan, bufferManager, modelManager)
	if err != nil {
		log.Printf("❌ %v", err)
	}

	// Initialize audio device if configured
	initAudioDevice(wg, settings, quitChan, bufferManager, modelManager)

	// Initialize RTSP streams if configured
	initRTSPStreams(settings, streamManager)

	// Register cleanup for the audio components
	registerCleanupHandler(wg, quitChan, func() {
		log.Println("Stopping stream manager...")
		streamManager.Stop()
		log.Println("Stopping FFmpeg monitor...")
		ffmpegMonitor.Stop()
	})

	return streamManager, ffmpegMonitor
}

// startClipCleanupMonitor initializes and starts the clip cleanup monitoring routine in a new goroutine.
func startClipCleanupMonitor(wg *sync.WaitGroup, quitChan chan struct{}, dataStore datastore.Interface) {
	registerCleanupHandler(wg, quitChan, func() {
		clipCleanupMonitor(quitChan, dataStore)
	})
}

// startWeatherPolling initializes and starts the weather polling routine in a new goroutine.
func startWeatherPolling(wg *sync.WaitGroup, settings *conf.Settings, dataStore datastore.Interface, quitChan chan struct{}) {
	// Create new weather service
	weatherService, err := weather.NewService(settings, dataStore)
	if err != nil {
		log.Printf("⛈️ Failed to initialize weather service: %v", err)
		return
	}

	registerCleanupHandler(wg, quitChan, func() {
		weatherService.StartPolling(quitChan)
	})
}

// startTelemetryEndpoint initializes and starts the telemetry endpoint if enabled
func startTelemetryEndpoint(wg *sync.WaitGroup, settings *conf.Settings, metrics *telemetry.Metrics, quitChan chan struct{}) {
	// Initialize Prometheus metrics endpoint if enabled
	if !settings.Realtime.Telemetry.Enabled {
		return
	}

	// Initialize metrics endpoint
	telemetryEndpoint, err := telemetry.NewEndpoint(settings, metrics)
	if err != nil {
		log.Printf("Error initializing telemetry endpoint: %v", err)
		return
	}

	// Start metrics server
	telemetryEndpoint.Start(wg, quitChan)
}

// monitorCtrlC listens for the SIGINT (Ctrl+C) signal and triggers the application shutdown process.
func monitorCtrlC(quitChan chan struct{}) {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT) // Register to receive SIGINT (Ctrl+C)

		<-sigChan // Block until a SIGINT signal is received

		log.Println("Received Ctrl+C, shutting down")
		close(quitChan) // Close the quit channel to signal other goroutines to stop
	}()
}

// closeDataStore attempts to close the database connection and logs the result.
func closeDataStore(store datastore.Interface) {
	if err := store.Close(); err != nil {
		log.Printf("Failed to close database: %v", err)
	} else {
		log.Println("Successfully closed database")
	}
}

// logOperation logs the result of an operation with standardized emojis
func logOperation(success bool, successMsg, errorMsg string, err error) error {
	if success {
		log.Printf("✅ %s", successMsg)
		return nil
	}
	log.Printf("❌ %s: %v", errorMsg, err)
	return err
}

// ClipCleanupMonitor monitors the database and deletes clips that meet the retention policy.
func clipCleanupMonitor(quitChan chan struct{}, dataStore datastore.Interface) {
	// Create a ticker that triggers every five minutes to perform cleanup
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop() // Ensure the ticker is stopped to prevent leaks

	log.Println("Clip retention policy:", conf.Setting().Realtime.Audio.Export.Retention.Policy)

	for {
		select {
		case <-quitChan:
			// Handle quit signal to stop the monitor
			return

		case <-ticker.C:
			log.Println("🧹 Running clip cleanup task")

			// age based cleanup method
			if conf.Setting().Realtime.Audio.Export.Retention.Policy == "age" {
				result := diskmanager.AgeBasedCleanup(quitChan, dataStore)
				if result.Err != nil {
					log.Printf("Error during age-based cleanup: %v", result.Err)
				} else {
					log.Printf("🧹 Age-based cleanup completed successfully, clips removed: %d, current disk utilization: %d%%", result.ClipsRemoved, result.DiskUtilization)
				}
			}

			// priority based cleanup method
			if conf.Setting().Realtime.Audio.Export.Retention.Policy == "usage" {
				result := diskmanager.UsageBasedCleanup(quitChan, dataStore)
				if result.Err != nil {
					log.Printf("Error during usage-based cleanup: %v", result.Err)
				} else {
					log.Printf("🧹 Usage-based cleanup completed successfully, clips removed: %d, current disk utilization: %d%%", result.ClipsRemoved, result.DiskUtilization)
				}
			}
		}
	}
}

// initBirdImageCache initializes the bird image cache by fetching all detected species from the database.
func initBirdImageCache(ds datastore.Interface, metrics *telemetry.Metrics) *imageprovider.BirdImageCache {
	// Create the cache first
	birdImageCache, err := imageprovider.CreateDefaultCache(metrics, ds)
	if err != nil {
		log.Printf("Failed to create image cache: %v", err)
		return nil
	}

	// Get the list of all detected species
	speciesList, err := ds.GetAllDetectedSpecies()
	if err != nil {
		log.Printf("Failed to get detected species: %v", err)
		return birdImageCache // Return the cache even if we can't get species list
	}

	// Start background fetching of images
	go func() {
		// Use a WaitGroup to wait for all goroutines to complete
		var wg sync.WaitGroup
		// Use a semaphore to limit concurrent fetches
		sem := make(chan struct{}, 5) // Limit to 5 concurrent fetches

		// Track how many species need images
		needsImage := 0

		for i := range speciesList {
			species := &speciesList[i] // Use pointer to avoid copying
			// Check if we already have this image cached
			if cached, err := ds.GetImageCache(species.ScientificName); err == nil && cached != nil {
				continue // Skip if already cached
			}

			needsImage++
			wg.Add(1)
			// Mark this species as being initialized
			birdImageCache.Initializing.Store(species.ScientificName, struct{}{})
			go func(name string) {
				defer func() {
					wg.Done()
				}()
				defer birdImageCache.Initializing.Delete(name) // Remove initialization mark when done
				sem <- struct{}{}                              // Acquire semaphore
				defer func() { <-sem }()                       // Release semaphore

				// Attempt to fetch the image for the given species
				if _, err := birdImageCache.Get(name); err != nil {
					log.Printf("Failed to fetch image for %s: %v", name, err)
				}
			}(species.ScientificName)
		}

		if needsImage > 0 {
			// Wait for all goroutines to complete
			wg.Wait()
			log.Printf("Finished initializing BirdImageCache (%d species needed images)", needsImage)
		} else {
			log.Println("BirdImageCache initialized (all species images already cached)")
		}
	}()

	return birdImageCache
}

// startControlMonitor handles various control signals for realtime analysis mode
func startControlMonitor(wg *sync.WaitGroup, controlChan chan string, quitChan, restartChan chan struct{},
	notificationChan chan handlers.Notification, proc *processor.Processor,
	streamManager audio.StreamManager, ffmpegMonitor audio.FFmpegMonitorInterface) {

	monitor := NewControlMonitor(wg, controlChan, quitChan, restartChan, notificationChan, proc, bufferManager)

	// Connect the stream components to the control monitor
	if streamManager != nil && ffmpegMonitor != nil {
		monitor.SetStreamComponents(streamManager, ffmpegMonitor)
	}

	monitor.Start()
}
