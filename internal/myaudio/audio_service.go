package myaudio

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
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

	// FFmpeg monitor for RTSP streams
	ffmpegMonitor *FFmpegMonitor
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
	s.malgoContext, err = malgo.InitContext([]malgo.Backend{backend}, malgo.ContextConfig{}, nil)
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

	// Initialize and start device capture if configured
	if s.settings.Realtime.Audio.Source != "" {
		if err := s.startLegacyDeviceCapture(); err != nil {
			return fmt.Errorf("failed to start device capture: %w", err)
		}
	}

	// Start multi-source capture
	// TODO: Update this when UI is updated to support multiple sources
	audioSources := s.getAudioSources()
	for _, source := range audioSources {
		if source.Enabled {
			if err := s.startDeviceCapture(source); err != nil {
				log.Printf("❌ Failed to start capture for device %s: %v", source.Name, err)
				continue
			}
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

		if decodedID == deviceID || infos[i].Name() == deviceID {
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

	// Initialize device config
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = conf.NumChannels
	deviceConfig.SampleRate = conf.SampleRate
	deviceConfig.Alsa.NoMMap = 1
	deviceConfig.Capture.DeviceID = source.Pointer

	// Initialize filter chain if equalizer is enabled
	var filterChain *equalizer.FilterChain
	if eqSettings.Enabled && len(eqSettings.Filters) > 0 {
		filterChain = equalizer.NewFilterChain()
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
	}

	// Data callback function
	onReceiveFrames := func(pSample2, pSamples []byte, framecount uint32) {
		// Apply audio EQ filters if enabled
		if filterChain != nil {
			err := ApplyFilters(pSamples)
			if err != nil {
				log.Printf("❌ Error applying audio filters: %v", err)
			}
		}

		// Write to buffers
		if err := WriteToAnalysisBuffer(sourceID, pSamples); err != nil {
			log.Printf("❌ Error writing to analysis buffer: %v", err)
		}

		if err := WriteToCaptureBuffer(sourceID, pSamples); err != nil {
			log.Printf("❌ Error writing to capture buffer: %v", err)
		}

		// Calculate audio level
		audioLevelData := calculateAudioLevel(pSamples, sourceID, source.Name)

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

	// Define stop device callback
	var deviceStopCallback func()
	deviceStopCallback = func() {
		// Check if the context is done
		select {
		case <-s.ctx.Done():
			// Service is shutting down, do not restart
			return
		default:
			// Wait before attempting to restart
			time.Sleep(100 * time.Millisecond)

			// Try to restart the device
			device, err := malgo.InitDevice(s.malgoContext.Context, deviceConfig, malgo.DeviceCallbacks{
				Data: onReceiveFrames,
				Stop: deviceStopCallback,
			})

			if err != nil {
				log.Printf("❌ Failed to reinitialize device %s: %v", source.Name, err)

				// Send restart signal
				select {
				case s.restartChan <- struct{}{}:
					// Successfully sent restart signal
				default:
					// Channel is full, give up
				}
				return
			}

			// Try to start the device
			if err := device.Start(); err != nil {
				log.Printf("❌ Failed to restart device %s: %v", source.Name, err)
				device.Uninit()

				// Send restart signal
				select {
				case s.restartChan <- struct{}{}:
					// Successfully sent restart signal
				default:
					// Channel is full, give up
				}
			} else {
				log.Printf("🔄 Device %s restarted successfully", source.Name)
			}
		}
	}

	// Device callbacks
	deviceCallbacks := malgo.DeviceCallbacks{
		Data: onReceiveFrames,
		Stop: deviceStopCallback,
	}

	// Initialize the device
	device, err := malgo.InitDevice(s.malgoContext.Context, deviceConfig, deviceCallbacks)
	if err != nil {
		log.Printf("❌ Failed to initialize device %s: %v", source.Name, err)
		return
	}

	// Start the device
	err = device.Start()
	if err != nil {
		log.Printf("❌ Failed to start device %s: %v", source.Name, err)

		// Clean up the device
		device.Uninit()
		return
	}

	log.Printf("🎤 Listening on source: %s (%s)", source.Name, source.ID)

	// Wait for the context to be done
	<-s.ctx.Done()

	// Stop and clean up the device
	_ = device.Stop()
	device.Uninit()

	log.Printf("🛑 Stopped listening on source: %s", source.Name)
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
