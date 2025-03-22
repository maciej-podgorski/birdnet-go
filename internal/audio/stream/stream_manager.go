package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Manager manages multiple stream sources
type Manager interface {
	// AddSource adds a stream source to the manager
	AddSource(source Source) error

	// RemoveSource removes a stream source from the manager
	RemoveSource(sourceID string) error

	// GetSource returns a stream source by ID
	GetSource(sourceID string) (Source, error)

	// ListSources returns all managed stream sources
	ListSources() []Source

	// Start starts the manager and all sources
	Start(ctx context.Context) error

	// Stop stops the manager and all sources
	Stop() error

	// SetCallbacks sets callbacks for stream events
	SetCallbacks(onData DataCallback, onLevel LevelCallback)
}

// DefaultManager implements the Manager interface
type DefaultManager struct {
	sources       map[string]Source
	sourcesMu     sync.RWMutex
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
	processorWg   sync.WaitGroup
	dataCallback  DataCallback
	levelCallback LevelCallback
}

// NewManager creates a new stream manager
func NewManager() *DefaultManager {
	return &DefaultManager{
		sources: make(map[string]Source),
	}
}

// SetCallbacks sets the callbacks for stream events
func (m *DefaultManager) SetCallbacks(onData DataCallback, onLevel LevelCallback) {
	m.dataCallback = onData
	m.levelCallback = onLevel
}

// AddSource adds a stream source to the manager
func (m *DefaultManager) AddSource(source Source) error {
	m.sourcesMu.Lock()
	defer m.sourcesMu.Unlock()

	if _, exists := m.sources[source.ID()]; exists {
		return fmt.Errorf("source with ID %s already exists", source.ID())
	}

	m.sources[source.ID()] = source

	// If manager is running, start the source
	if m.running {
		return m.startSource(source)
	}

	return nil
}

// RemoveSource removes a stream source from the manager
func (m *DefaultManager) RemoveSource(sourceID string) error {
	m.sourcesMu.Lock()
	defer m.sourcesMu.Unlock()

	source, exists := m.sources[sourceID]
	if !exists {
		return fmt.Errorf("source with ID %s not found", sourceID)
	}

	// If running, stop the source
	if m.running && source.IsActive() {
		if err := source.Stop(); err != nil {
			return fmt.Errorf("failed to stop source: %w", err)
		}
	}

	delete(m.sources, sourceID)
	return nil
}

// GetSource returns a stream source by ID
func (m *DefaultManager) GetSource(sourceID string) (Source, error) {
	m.sourcesMu.RLock()
	defer m.sourcesMu.RUnlock()

	source, exists := m.sources[sourceID]
	if !exists {
		return nil, fmt.Errorf("source with ID %s not found", sourceID)
	}

	return source, nil
}

// ListSources returns all managed stream sources
func (m *DefaultManager) ListSources() []Source {
	m.sourcesMu.RLock()
	defer m.sourcesMu.RUnlock()

	sources := make([]Source, 0, len(m.sources))
	for _, source := range m.sources {
		sources = append(sources, source)
	}

	return sources
}

// Start starts the manager and all sources
func (m *DefaultManager) Start(ctx context.Context) error {
	m.sourcesMu.Lock()
	defer m.sourcesMu.Unlock()

	if m.running {
		return errors.New("manager already running")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true

	// Start all sources
	for _, source := range m.sources {
		if err := m.startSource(source); err != nil {
			// Log error but continue with other sources
			fmt.Printf("Error starting source %s: %v\n", source.ID(), err)
		}
	}

	return nil
}

// Stop stops the manager and all sources
func (m *DefaultManager) Stop() error {
	m.sourcesMu.Lock()

	if !m.running {
		m.sourcesMu.Unlock()
		return nil
	}

	// Cancel the context to signal all processors to stop
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all sources
	for _, source := range m.sources {
		if source.IsActive() {
			_ = source.Stop() // Ignore errors during stop
		}
	}

	m.running = false
	m.sourcesMu.Unlock()

	// Wait for all processors to finish
	m.processorWg.Wait()

	return nil
}

// startSource starts a single source and its processing
func (m *DefaultManager) startSource(source Source) error {
	reader, err := source.Start(m.ctx)
	if err != nil {
		return err
	}

	// Start processing in background
	m.processorWg.Add(1)
	go func() {
		defer m.processorWg.Done()
		defer reader.Close()

		m.processStream(reader, source.ID(), source.Name())
	}()

	return nil
}

// processStream reads data from the stream and calls the callbacks
func (m *DefaultManager) processStream(reader io.Reader, sourceID, sourceName string) {
	buf := make([]byte, 32768)

	for {
		// Check if context is cancelled
		select {
		case <-m.ctx.Done():
			return
		default:
			// Continue processing
		}

		// Read data from stream
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from stream %s: %v\n", sourceID, err)
			}
			return
		}

		if n <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		data := buf[:n]

		// Call the data callback if set
		if m.dataCallback != nil {
			m.dataCallback(sourceID, data)
		}

		// Calculate and report audio level if callback is set
		if m.levelCallback != nil {
			level := calculateAudioLevel(data, sourceID, sourceName)
			m.levelCallback(level)
		}
	}
}

// calculateAudioLevel calculates the audio level from raw PCM data
func calculateAudioLevel(data []byte, sourceID, sourceName string) AudioLevelData {
	// Simple implementation - can be improved for better accuracy
	if len(data) == 0 {
		return AudioLevelData{Level: 0, Clipping: false, Source: sourceID, Name: sourceName}
	}

	// Assuming 16-bit PCM data for simplicity
	// This should be adapted based on your actual data format
	maxLevel := 0
	clipping := false

	// Process every other byte (16-bit samples)
	for i := 0; i < len(data)-1; i += 2 {
		// Convert bytes to 16-bit sample
		sample := int(int16(data[i]) | int16(data[i+1])<<8)
		if sample < 0 {
			sample = -sample
		}

		if sample > maxLevel {
			maxLevel = sample
		}

		// Check for clipping (assuming 16-bit audio)
		if sample > 32700 { // Close to maximum 16-bit value
			clipping = true
		}
	}

	// Convert to percentage scale (0-100)
	level := (maxLevel * 100) / 32768

	return AudioLevelData{
		Level:    level,
		Clipping: clipping,
		Source:   sourceID,
		Name:     sourceName,
	}
}
