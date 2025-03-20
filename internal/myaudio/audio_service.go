package myaudio

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/myaudio/equalizer"
)

// AudioContext defines the interface for audio context operations
type AudioContext interface {
	Devices(deviceType malgo.DeviceType) ([]malgo.DeviceInfo, error)
	InitDevice(config malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (*malgo.Device, error)
	Uninit() error
}

// AudioDevice defines the interface for audio device operations
type AudioDevice interface {
	Start() error
	Stop() error
	Uninit() error
}

// AudioCaptureService manages the capture of audio from multiple sources
type AudioCaptureService struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             *sync.WaitGroup
	settings       *conf.Settings
	malgoContext   *malgo.AllocatedContext
	audioLevelChan chan AudioLevelData
	restartChan    chan struct{}
	bn             *birdnet.BirdNET // BirdNET instance for analysis

	// FFmpeg monitor for RTSP streams
	ffmpegMonitor *FFmpegMonitor

	// Monitors for analysis buffers
	bufferMonitors   map[string]chan struct{}
	bufferMonitorsMu sync.Mutex
}

// deviceState tracks the state of a capture device
type deviceState struct {
	device      *malgo.Device
	deviceInfo  captureSource
	filterChain *equalizer.FilterChain
	isActive    bool
}

// NewAudioCaptureService creates a new audio capture service
func NewAudioCaptureService(settings *conf.Settings) (*AudioCaptureService, error) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	service := &AudioCaptureService{
		ctx:            ctx,
		cancel:         cancel,
		wg:             wg,
		settings:       settings,
		audioLevelChan: make(chan AudioLevelData, 10),
		restartChan:    make(chan struct{}, 10),
		bufferMonitors: make(map[string]chan struct{}),
	}

	// Initialize the malgo context
	if err := service.initMalgoContext(); err != nil {
		return nil, fmt.Errorf("failed to initialize audio context: %w", err)
	}

	return service, nil
}

// initMalgoContext initializes the malgo context
func (s *AudioCaptureService) initMalgoContext() error {
	// If Linux set malgo.BackendAlsa, else set nil for auto select
	var backend malgo.Backend
	switch runtime.GOOS {
	case "linux":
		backend = malgo.BackendAlsa
	case "windows":
		backend = malgo.BackendWasapi
	case "darwin":
		backend = malgo.BackendCoreaudio
	}

	var err error
	s.malgoContext, err = malgo.InitContext([]malgo.Backend{backend}, malgo.ContextConfig{}, func(message string) {
		if s.settings.Debug {
			fmt.Print(message)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to initialize malgo context: %w", err)
	}

	return nil
}

// Start starts audio capture from all enabled sources
func (s *AudioCaptureService) Start() error {
	// Initialize and start RTSP streams if configured
	if len(s.settings.Realtime.RTSP.URLs) > 0 {
		if err := s.startRTSPCapture(); err != nil {
			return fmt.Errorf("failed to start RTSP capture: %w", err)
		}
	}

	// Get multi-source configurations
	audioSources := s.getAudioSources()

	// Start multi-source capture if any sources are defined
	if len(audioSources) > 0 {
		for _, source := range audioSources {
			if source.Enabled {
				if err := s.startDeviceCapture(source); err != nil {
					log.Printf("❌ Failed to start capture for device %s: %v", source.Name, err)
					continue
				}
			}
		}
	} else if s.settings.Realtime.Audio.Source != "" {
		// Fall back to legacy device capture only if no sources are defined
		if err := s.startLegacyDeviceCapture(); err != nil {
			return fmt.Errorf("failed to start device capture: %w", err)
		}
	}

	return nil
}

// Stop stops all audio capture
func (s *AudioCaptureService) Stop() {
	// Cancel the context to signal all goroutines to stop
	s.cancel()

	// Stop the FFmpeg monitor if running
	if s.ffmpegMonitor != nil {
		s.ffmpegMonitor.Stop()
		s.ffmpegMonitor = nil
	}

	// Stop all buffer monitors
	s.stopAllBufferMonitors()

	// Wait for all goroutines to complete
	s.wg.Wait()

	// Clean up the malgo context
	if s.malgoContext != nil {
		if err := s.malgoContext.Uninit(); err != nil {
			log.Printf("❌ Failed to uninitialize malgo context: %v", err)
		}
	}
}

// AllocateBuffersForSource allocates analysis and capture buffers for a source
func (s *AudioCaptureService) AllocateBuffersForSource(sourceID string) error {
	// Check if analysis buffer exists
	abMutex.RLock()
	_, abExists := analysisBuffers[sourceID]
	abMutex.RUnlock()

	// Check if capture buffer exists
	cbMutex.RLock()
	_, cbExists := captureBuffers[sourceID]
	cbMutex.RUnlock()

	// Initialize analysis buffer if it doesn't exist
	if !abExists {
		if err := AllocateAnalysisBuffer(conf.BufferSize*3, sourceID); err != nil {
			return fmt.Errorf("failed to create analysis buffer: %w", err)
		}
	}

	// Initialize capture buffer if it doesn't exist
	if !cbExists {
		if err := AllocateCaptureBuffer(60, conf.SampleRate, conf.BitDepth/8, sourceID); err != nil {
			// Clean up the analysis buffer if we just created it
			if !abExists {
				if cleanupErr := RemoveAnalysisBuffer(sourceID); cleanupErr != nil {
					log.Printf("❌ Failed to cleanup analysis buffer after capture buffer init failure for %s: %v", sourceID, cleanupErr)
				}
			}
			return fmt.Errorf("failed to create capture buffer: %w", err)
		}
	}

	// Start the buffer monitor if we have a BirdNET instance
	if s.bn != nil {
		s.StartBufferMonitor(sourceID)
	}

	return nil
}

// RemoveBuffersForSource removes analysis and capture buffers for a source
func (s *AudioCaptureService) RemoveBuffersForSource(sourceID string) error {
	// Remove analysis buffer
	if err := RemoveAnalysisBuffer(sourceID); err != nil {
		log.Printf("❌ Warning: failed to remove analysis buffer for %s: %v", sourceID, err)
	}

	// Remove capture buffer
	if err := RemoveCaptureBuffer(sourceID); err != nil {
		log.Printf("❌ Warning: failed to remove capture buffer for %s: %v", sourceID, err)
	}

	return nil
}

// GetAudioLevelChannel returns the channel for audio level data
func (s *AudioCaptureService) GetAudioLevelChannel() <-chan AudioLevelData {
	return s.audioLevelChan
}

// startLegacyDeviceCapture starts capture from the legacy device setting
func (s *AudioCaptureService) startLegacyDeviceCapture() error {
	// Validate the audio device
	if err := ValidateAudioDevice(s.settings); err != nil {
		return fmt.Errorf("audio device validation failed: %w", err)
	}

	// Print available capture sources
	if err := s.printAvailableSources(); err != nil {
		log.Printf("⚠️ Unable to list audio devices: %v", err)
	}

	// Select the capture source
	source, err := selectCaptureSource(s.settings)
	if err != nil {
		return fmt.Errorf("failed to select capture source: %w", err)
	}

	// Allocate buffers for the source
	if err := s.AllocateBuffersForSource("malgo"); err != nil {
		return fmt.Errorf("failed to allocate buffers: %w", err)
	}

	// Start capture
	s.wg.Add(1)
	go s.captureFromDevice("malgo", source, s.settings.Realtime.Audio.Equalizer)

	return nil
}

// startDeviceCapture starts capture from a specific device
func (s *AudioCaptureService) startDeviceCapture(source conf.AudioSourceConfig) error {
	// Generate a unique source ID
	sourceID := fmt.Sprintf("malgo:%s", source.ID)

	// Print available capture sources
	if err := s.printAvailableSources(); err != nil {
		log.Printf("⚠️ Unable to list audio devices: %v", err)
	}

	// Select the capture source
	deviceSource, err := s.findDeviceByID(source.ID)
	if err != nil {
		return fmt.Errorf("failed to find device: %w", err)
	}

	// Allocate buffers for the source
	if err := s.AllocateBuffersForSource(sourceID); err != nil {
		return fmt.Errorf("failed to allocate buffers: %w", err)
	}

	// Start capture
	s.wg.Add(1)
	go s.captureFromDevice(sourceID, deviceSource, source.Equalizer)

	return nil
}

// printAvailableSources prints a list of available audio capture sources
func (s *AudioCaptureService) printAvailableSources() error {
	// Get a list of capture devices
	infos, err := s.malgoContext.Devices(malgo.Capture)
	if err != nil {
		return fmt.Errorf("failed to get capture devices: %w", err)
	}

	fmt.Println("Available Capture Sources:")
	for i := range infos {
		decodedID, err := hexToASCII(infos[i].ID.String())
		if err != nil {
			fmt.Printf("❌ Error decoding ID for device %d: %v\n", i, err)
			continue
		}

		output := fmt.Sprintf("  %d: %s", i, infos[i].Name())
		if runtime.GOOS == "linux" {
			output = fmt.Sprintf("%s, %s", output, decodedID)
		}

		// Check if this device matches the configured source
		deviceMatches := s.settings.Realtime.Audio.Source != "" &&
			(decodedID == s.settings.Realtime.Audio.Source ||
				strings.Contains(infos[i].Name(), s.settings.Realtime.Audio.Source))

		if deviceMatches {
			// Create a test config to see if the device works
			deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
			deviceConfig.Capture.Format = malgo.FormatS16
			deviceConfig.Capture.Channels = conf.NumChannels
			deviceConfig.SampleRate = conf.SampleRate
			deviceConfig.Alsa.NoMMap = 1
			deviceConfig.Periods = 2
			deviceConfig.Capture.DeviceID = infos[i].ID.Pointer()

			// Try to initialize the device
			device, err := malgo.InitDevice(s.malgoContext.Context, deviceConfig, malgo.DeviceCallbacks{})
			if err == nil {
				// Device initialized successfully
				fmt.Printf("%s (✅ selected)\n", output)
				// Uninitialize the device to free resources
				device.Uninit()
				continue
			}
			// Device initialization failed
			fmt.Printf("%s (❌ device test failed: %v)\n", output, err)
			continue
		}

		fmt.Println(output)
	}

	return nil
}

// findDeviceByID finds a device by its ID
func (s *AudioCaptureService) findDeviceByID(deviceID string) (captureSource, error) {
	// Get a list of capture devices
	infos, err := s.malgoContext.Devices(malgo.Capture)
	if err != nil {
		return captureSource{}, fmt.Errorf("failed to get devices: %w", err)
	}

	// Find the device with the matching ID
	for i := range infos {
		decodedID, err := hexToASCII(infos[i].ID.String())
		if err != nil {
			continue
		}

		// More flexible matching - accept partial matches
		if decodedID == deviceID ||
			infos[i].Name() == deviceID ||
			strings.Contains(infos[i].Name(), deviceID) ||
			strings.Contains(decodedID, deviceID) {
			return captureSource{
				Name:    infos[i].Name(),
				ID:      decodedID,
				Pointer: infos[i].ID.Pointer(),
			}, nil
		}
	}

	return captureSource{}, fmt.Errorf("device not found: %s", deviceID)
}

// captureFromDevice captures audio from a specific device
func (s *AudioCaptureService) captureFromDevice(sourceID string, source captureSource, eqSettings conf.EqualizerSettings) {
	defer s.wg.Done()

	// Create device configuration
	deviceConfig := s.createDeviceConfig(source)

	// Create filter chain
	filterChain := s.createFilterChain(eqSettings)

	// Used to store device reference globally for the function
	var captureDevice *malgo.Device

	// Create callbacks
	onDataCallback := s.createDataCallback(sourceID, source.Name, filterChain)
	onStopCallback := s.createStopCallback(source.Name, &captureDevice)

	// Initialize and start the device
	var err error
	captureDevice, err = s.initializeDevice(&deviceConfig, onDataCallback, onStopCallback)
	if err != nil {
		log.Printf("❌ Failed to initialize device %s: %v", source.Name, err)
		return
	}

	if err = captureDevice.Start(); err != nil {
		log.Printf("❌ Failed to start device %s: %v", source.Name, err)
		captureDevice.Uninit()
		captureDevice = nil
		return
	}

	// Start the buffer monitor for this source
	s.StartBufferMonitor(sourceID)

	// Log device start
	if s.settings.Debug {
		log.Printf("🎤 Debug mode: starting to listen on source: %s (%s)", source.Name, source.ID)
	} else {
		log.Printf("🎤 Listening on source: %s (%s)", source.Name, source.ID)
	}

	// Wait for the context to be done
	<-s.ctx.Done()

	// Clean up the device
	if captureDevice != nil {
		_ = captureDevice.Stop()
		captureDevice.Uninit()
		captureDevice = nil
	}
	log.Printf("🛑 Stopped listening on source: %s", source.Name)
}

// createDeviceConfig creates a device configuration
func (s *AudioCaptureService) createDeviceConfig(source captureSource) malgo.DeviceConfig {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = conf.NumChannels
	deviceConfig.SampleRate = conf.SampleRate
	deviceConfig.Alsa.NoMMap = 1
	deviceConfig.Capture.DeviceID = source.Pointer
	return deviceConfig
}

// createFilterChain creates a filter chain for the device
func (s *AudioCaptureService) createFilterChain(eqSettings conf.EqualizerSettings) *equalizer.FilterChain {
	if !eqSettings.Enabled || len(eqSettings.Filters) == 0 {
		return nil
	}

	filterChain := equalizer.NewFilterChain()
	for _, filterConfig := range eqSettings.Filters {
		filter, err := createFilter(filterConfig, float64(conf.SampleRate))
		if err != nil {
			log.Printf("❌ Failed to create filter: %v", err)
			continue
		}
		if filter != nil {
			if err := filterChain.AddFilter(filter); err != nil {
				log.Printf("❌ Failed to add filter: %v", err)
			}
		}
	}
	return filterChain
}

// createDataCallback creates the data callback function
func (s *AudioCaptureService) createDataCallback(sourceID, sourceName string, filterChain *equalizer.FilterChain) malgo.DataProc {
	return func(pOutput, pInput []byte, framecount uint32) {
		// Apply audio EQ filters if enabled
		if filterChain != nil {
			if err := ApplyFilters(pInput); err != nil {
				log.Printf("❌ Error applying audio filters: %v", err)
			}
		}

		// Write to buffers
		if err := WriteToAnalysisBuffer(sourceID, pInput); err != nil {
			log.Printf("❌ Error writing to analysis buffer: %v", err)
		}

		if err := WriteToCaptureBuffer(sourceID, pInput); err != nil {
			log.Printf("❌ Error writing to capture buffer: %v", err)
		}

		// Calculate audio level
		audioLevelData := calculateAudioLevel(pInput, sourceID, sourceName)
		s.sendAudioLevel(audioLevelData)
	}
}

// sendAudioLevel sends audio level data to the channel
func (s *AudioCaptureService) sendAudioLevel(audioLevelData AudioLevelData) {
	// Send level to channel (non-blocking)
	select {
	case s.audioLevelChan <- audioLevelData:
		// Data sent successfully
	default:
		// Channel is full, clear the channel
		for len(s.audioLevelChan) > 0 {
			<-s.audioLevelChan
		}
		// Try to send the new data
		select {
		case s.audioLevelChan <- audioLevelData:
			// Successfully sent
		default:
			// Channel is still full, give up
		}
	}
}

// createStopCallback creates the stop callback function
func (s *AudioCaptureService) createStopCallback(sourceName string, devicePtr **malgo.Device) malgo.StopProc {
	return func() {
		go func() {
			select {
			case <-s.ctx.Done():
				// Service is shutting down, do not restart
				return
			case <-time.After(100 * time.Millisecond):
				s.handleDeviceRestart(sourceName, devicePtr)
			}
		}()
	}
}

// handleDeviceRestart attempts to restart a device after it stops
func (s *AudioCaptureService) handleDeviceRestart(sourceName string, devicePtr **malgo.Device) {
	// Wait a bit before restarting to avoid potential rapid restart loops
	if s.settings.Debug {
		log.Printf("🔄 Attempting to restart audio device %s", sourceName)
	}

	device := *devicePtr
	if device == nil {
		return
	}

	err := device.Start()
	if err != nil {
		log.Printf("❌ Failed to restart audio device %s: %v", sourceName, err)
		log.Println("🔄 Attempting full audio context restart in 1 second.")
		time.Sleep(1 * time.Second)

		// Send restart signal
		select {
		case s.restartChan <- struct{}{}:
			// Successfully sent restart signal
		case <-s.ctx.Done():
			// Service is shutting down, don't send restart signal
		}
	} else if s.settings.Debug {
		log.Printf("🔄 Audio device %s restarted successfully.", sourceName)
	}
}

// initializeDevice initializes a new audio device
func (s *AudioCaptureService) initializeDevice(config *malgo.DeviceConfig, dataCallback malgo.DataProc, stopCallback malgo.StopProc) (*malgo.Device, error) {
	callbacks := malgo.DeviceCallbacks{
		Data: dataCallback,
		Stop: stopCallback,
	}

	return malgo.InitDevice(s.malgoContext.Context, *config, callbacks)
}

// getAudioSources returns the list of audio sources, converting legacy format if needed
func (s *AudioCaptureService) getAudioSources() []conf.AudioSourceConfig {
	// New sources array first
	var audioSources []conf.AudioSourceConfig

	// For backward compatibility with existing config files
	// If a single source is configured but no sources array, create a source from it
	if s.settings.Realtime.Audio.Source != "" && len(audioSources) == 0 {
		// Create a source configuration from the legacy single source
		legacySource := conf.AudioSourceConfig{
			ID:        s.settings.Realtime.Audio.Source,
			Name:      s.settings.Realtime.Audio.Source,
			Enabled:   true,
			Equalizer: s.settings.Realtime.Audio.Equalizer,
		}
		audioSources = append(audioSources, legacySource)
	}

	return audioSources
}

// ConnectExternalChannels connects external channels to the service's internal channels
func (s *AudioCaptureService) ConnectExternalChannels(restartChan chan struct{}) {
	// Connect restart channel if provided
	if restartChan != nil {
		go func() {
			for {
				select {
				case <-s.ctx.Done():
					return
				case <-s.restartChan:
					// Forward restart signals to the external channel
					select {
					case restartChan <- struct{}{}:
					default:
						// If external channel is full, we just drop the signal
					}
				}
			}
		}()
	}
}

// SetBirdNET sets the BirdNET instance for analysis
func (s *AudioCaptureService) SetBirdNET(bn *birdnet.BirdNET) {
	s.bn = bn
}

// StartBufferMonitor starts a monitor for the specified buffer
func (s *AudioCaptureService) StartBufferMonitor(sourceID string) {
	if s.bn == nil {
		log.Printf("❌ Cannot start buffer monitor for %s: BirdNET instance not set", sourceID)
		return
	}

	s.bufferMonitorsMu.Lock()
	defer s.bufferMonitorsMu.Unlock()

	// Check if monitor already exists
	if _, exists := s.bufferMonitors[sourceID]; exists {
		// Monitor already running
		return
	}

	// Create a new quit channel for this monitor
	quitChan := make(chan struct{})
	s.bufferMonitors[sourceID] = quitChan

	// Start the monitor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("📊 Started analysis buffer monitor for %s", sourceID)
		AnalysisBufferMonitor(s.wg, s.bn, quitChan, sourceID)
	}()
}

// StopBufferMonitor stops a monitor for the specified buffer
func (s *AudioCaptureService) StopBufferMonitor(sourceID string) {
	s.bufferMonitorsMu.Lock()
	defer s.bufferMonitorsMu.Unlock()

	if quitChan, exists := s.bufferMonitors[sourceID]; exists {
		close(quitChan)
		delete(s.bufferMonitors, sourceID)
		log.Printf("🛑 Stopped analysis buffer monitor for %s", sourceID)
	}
}

// stopAllBufferMonitors stops all active buffer monitors
func (s *AudioCaptureService) stopAllBufferMonitors() {
	s.bufferMonitorsMu.Lock()
	defer s.bufferMonitorsMu.Unlock()

	for sourceID, quitChan := range s.bufferMonitors {
		close(quitChan)
		log.Printf("🛑 Stopped analysis buffer monitor for %s", sourceID)
	}

	// Clear the map
	s.bufferMonitors = make(map[string]chan struct{})
}

// startRTSPCapture starts capture from RTSP streams
func (s *AudioCaptureService) startRTSPCapture() error {
	// Initialize FFmpeg monitor if not already running
	if s.ffmpegMonitor == nil {
		s.ffmpegMonitor = NewDefaultFFmpegMonitor()
		s.ffmpegMonitor.Start()
	}

	// Start capture from each RTSP URL
	for _, url := range s.settings.Realtime.RTSP.URLs {
		// Allocate buffers for the source
		if err := s.AllocateBuffersForSource(url); err != nil {
			log.Printf("❌ Failed to allocate buffers for RTSP source %s: %v", url, err)
			continue
		}

		// Start capture
		s.wg.Add(1)
		go s.captureRTSPWithService(url, s.settings.Realtime.RTSP.Transport)
	}

	return nil
}

// captureRTSPWithService captures audio from an RTSP stream
func (s *AudioCaptureService) captureRTSPWithService(url, transport string) {
	defer s.wg.Done()

	// Create a quit channel from the context
	quitChan := make(chan struct{})
	go func() {
		<-s.ctx.Done()
		close(quitChan)
	}()

	// Call the existing CaptureAudioRTSP function with our channels
	CaptureAudioRTSP(url, transport, s.wg, quitChan, s.restartChan, s.audioLevelChan)
}
