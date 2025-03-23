package analysis

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tphakala/birdnet-go/internal/analysis/processor"
	"github.com/tphakala/birdnet-go/internal/audio"
	"github.com/tphakala/birdnet-go/internal/audio/buffer"
	"github.com/tphakala/birdnet-go/internal/birdnet"
	"github.com/tphakala/birdnet-go/internal/birdweather"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/httpcontroller/handlers"
	"github.com/tphakala/birdnet-go/internal/mqtt"
)

// ControlMonitor handles control signals for realtime analysis mode
type ControlMonitor struct {
	wg               *sync.WaitGroup
	controlChan      chan string
	quitChan         chan struct{}
	restartChan      chan struct{}
	notificationChan chan handlers.Notification
	proc             *processor.Processor
	audioLevelChan   chan audio.AudioLevelData
	bn               *birdnet.BirdNET
	streamManager    audio.StreamManager
	ffmpegMonitor    audio.FFmpegMonitorInterface
	bufferManager    buffer.BufferManagerInterface
}

// NewControlMonitor creates a new ControlMonitor instance
func NewControlMonitor(wg *sync.WaitGroup, controlChan chan string, quitChan, restartChan chan struct{}, notificationChan chan handlers.Notification, proc *processor.Processor, bufferManager buffer.BufferManagerInterface) *ControlMonitor {
	return &ControlMonitor{
		wg:               wg,
		controlChan:      controlChan,
		quitChan:         quitChan,
		restartChan:      restartChan,
		notificationChan: notificationChan,
		proc:             proc,
		audioLevelChan:   make(chan audio.AudioLevelData),
		bn:               proc.Bn,
		bufferManager:    bufferManager,
	}
}

// SetStreamComponents sets the audio streaming components for this monitor
func (cm *ControlMonitor) SetStreamComponents(streamManager audio.StreamManager, ffmpegMonitor audio.FFmpegMonitorInterface) {
	cm.streamManager = streamManager
	cm.ffmpegMonitor = ffmpegMonitor
}

// Start begins monitoring control signals
func (cm *ControlMonitor) Start() {
	go cm.monitor()
}

// monitor listens for control signals and handles them
func (cm *ControlMonitor) monitor() {
	for {
		select {
		case signal := <-cm.controlChan:
			cm.handleControlSignal(signal)
		case <-cm.quitChan:
			return
		}
	}
}

// handleControlSignal processes different control signals
func (cm *ControlMonitor) handleControlSignal(signal string) {
	switch signal {
	case "rebuild_range_filter":
		cm.handleRebuildRangeFilter()
	case "reload_birdnet":
		cm.handleReloadBirdnet()
	case "reconfigure_mqtt":
		cm.handleReconfigureMQTT()
	case "reconfigure_rtsp_sources":
		cm.handleReconfigureRTSP()
	case "reconfigure_birdweather":
		cm.handleReconfigureBirdWeather()
	default:
		log.Printf("Received unknown control signal: %v", signal)
	}
}

// handleRebuildRangeFilter rebuilds the range filter
func (cm *ControlMonitor) handleRebuildRangeFilter() {
	if err := birdnet.BuildRangeFilter(cm.bn); err != nil {
		log.Printf("\033[31m❌ Error handling range filter rebuild: %v\033[0m", err)
		cm.notifyError("Failed to rebuild range filter", err)
	} else {
		log.Printf("\033[32m🔄 Range filter rebuilt successfully\033[0m")
		cm.notifySuccess("Range filter rebuilt successfully")
	}
}

// handleReloadBirdnet reloads the BirdNET model
func (cm *ControlMonitor) handleReloadBirdnet() {
	if err := cm.bn.ReloadModel(); err != nil {
		log.Printf("\033[31m❌ Error reloading BirdNET model: %v\033[0m", err)
		cm.notifyError("Failed to reload BirdNET model", err)
		return
	}

	log.Printf("\033[32m✅ BirdNET model reloaded successfully\033[0m")
	cm.notifySuccess("BirdNET model reloaded successfully")

	// Rebuild range filter after model reload
	if err := birdnet.BuildRangeFilter(cm.bn); err != nil {
		log.Printf("\033[31m❌ Error rebuilding range filter after model reload: %v\033[0m", err)
		cm.notifyError("Failed to rebuild range filter", err)
	} else {
		log.Printf("\033[32m✅ Range filter rebuilt successfully\033[0m")
		cm.notifySuccess("Range filter rebuilt successfully")
	}
}

// handleReconfigureMQTT reconfigures the MQTT connection
func (cm *ControlMonitor) handleReconfigureMQTT() {
	log.Printf("\033[32m🔄 Reconfiguring MQTT connection...\033[0m")
	settings := conf.Setting()

	if cm.proc == nil {
		log.Printf("\033[31m❌ Error: Processor not available\033[0m")
		cm.notifyError("Failed to reconfigure MQTT", fmt.Errorf("processor not available"))
		return
	}

	// First, safely disconnect any existing client
	cm.proc.DisconnectMQTTClient()

	// If MQTT is enabled, initialize and connect
	if settings.Realtime.MQTT.Enabled {
		var err error
		newClient, err := mqtt.NewClient(settings, cm.proc.Metrics)
		if err != nil {
			log.Printf("\033[31m❌ Error creating MQTT client: %v\033[0m", err)
			cm.notifyError("Failed to create MQTT client", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := newClient.Connect(ctx); err != nil {
			cancel()
			log.Printf("\033[31m❌ Error connecting to MQTT broker: %v\033[0m", err)
			cm.notifyError("Failed to connect to MQTT broker", err)
			return
		}
		cancel()

		// Safely set the new client
		cm.proc.SetMQTTClient(newClient)

		log.Printf("\033[32m✅ MQTT connection configured successfully\033[0m")
		cm.notifySuccess("MQTT connection configured successfully")
	} else {
		log.Printf("\033[32m✅ MQTT connection disabled\033[0m")
		cm.notifySuccess("MQTT connection disabled")
	}
}

// handleReconfigureRTSP reconfigures RTSP sources
func (cm *ControlMonitor) handleReconfigureRTSP() {
	log.Printf("\033[32m🔄 Reconfiguring audio sources...\033[0m")
	settings := conf.Setting()

	// Check if we have the new StreamManager
	if cm.streamManager != nil {
		// Get list of current active streams before stopping them
		activeStreams := cm.streamManager.GetActiveStreams()

		// Stop all existing streams
		cm.streamManager.StopAllStreams()

		// Track which sources will remain active
		newActiveSources := make(map[string]bool)

		// Add RTSP streams if configured
		if len(settings.Realtime.RTSP.URLs) > 0 {
			for _, url := range settings.Realtime.RTSP.URLs {
				if url == "" {
					continue
				}

				transport := settings.Realtime.RTSP.Transport
				if transport == "" {
					transport = "tcp" // Default to TCP if not specified
				}

				// Mark this source as active in the new configuration
				sourceID := url
				newActiveSources[sourceID] = true

				if err := cm.streamManager.StartStream(url, transport); err != nil {
					log.Printf("\033[31m❌ Error starting stream %s: %v\033[0m", url, err)
				} else {
					log.Printf("\033[32m✅ Started stream: %s\033[0m", url)
				}
			}
		}

		// Cleanup buffers for sources that were removed
		if cm.bufferManager != nil {
			for _, sourceID := range activeStreams {
				// Skip if source is still active in new configuration
				if newActiveSources[sourceID] {
					continue
				}

				// Source was removed, deallocate its buffers
				log.Printf("\033[33m🧹 Removing buffers for inactive source: %s\033[0m", sourceID)

				// Remove all buffers for this source
				RemoveAllBuffersForSource(cm.bufferManager, sourceID)
			}
		}

		log.Printf("\033[32m✅ Audio sources reconfigured successfully\033[0m")
		cm.notifySuccess("Audio sources reconfigured successfully")
		return
	}

	// If StreamManager is not available, use restart channel as fallback
	log.Printf("\033[33m⚠️ StreamManager not available, using restart signal\033[0m")

	// Signal restart to any listening components
	select {
	case cm.restartChan <- struct{}{}:
		log.Printf("\033[32m✅ Restart signal sent\033[0m")
	default:
		log.Printf("\033[33m⚠️ No listeners for restart signal\033[0m")
	}

	log.Printf("\033[32m✅ Audio sources reconfiguration requested\033[0m")
	cm.notifySuccess("Audio sources reconfiguration requested")
}

// handleReconfigureBirdWeather reconfigures the BirdWeather integration
func (cm *ControlMonitor) handleReconfigureBirdWeather() {
	log.Printf("\033[32m🔄 Reconfiguring BirdWeather integration...\033[0m")
	settings := conf.Setting()

	if cm.proc == nil {
		log.Printf("\033[31m❌ Error: Processor not available\033[0m")
		cm.notifyError("Failed to reconfigure BirdWeather", fmt.Errorf("processor not available"))
		return
	}

	// First, safely disconnect any existing client
	cm.proc.DisconnectBwClient()

	// If BirdWeather is enabled, initialize and connect
	if settings.Realtime.Birdweather.Enabled {
		var err error
		newClient, err := birdweather.New(settings)
		if err != nil {
			log.Printf("\033[31m❌ Error creating BirdWeather client: %v\033[0m", err)
			cm.notifyError("Failed to create BirdWeather client", err)
			return
		}

		// Safely set the new client
		cm.proc.SetBwClient(newClient)

		log.Printf("\033[32m✅ BirdWeather integration configured successfully\033[0m")
		cm.notifySuccess("BirdWeather integration configured successfully")
	} else {
		log.Printf("\033[32m✅ BirdWeather integration disabled\033[0m")
		cm.notifySuccess("BirdWeather integration disabled")
	}
}

// notifySuccess sends a success notification
func (cm *ControlMonitor) notifySuccess(message string) {
	if cm.notificationChan != nil {
		cm.notificationChan <- handlers.Notification{Type: "success", Message: message}
	}
}

// notifyError sends an error notification
func (cm *ControlMonitor) notifyError(message string, err error) {
	if cm.notificationChan != nil {
		errorMsg := fmt.Sprintf("%s: %v", message, err)
		cm.notificationChan <- handlers.Notification{Type: "error", Message: errorMsg}
	}
}
