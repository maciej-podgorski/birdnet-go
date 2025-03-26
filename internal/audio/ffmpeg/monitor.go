package ffmpeg

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProcessInfo contains information about a system process
type ProcessInfo struct {
	PID  int
	Name string
}

// ProcessTracker tracks FFmpeg processes
type ProcessTracker interface {
	// RegisterProcess registers a process with a URL
	RegisterProcess(url string, process *Process)

	// UnregisterProcess removes a process registration
	UnregisterProcess(url string)

	// GetProcess returns a process by URL
	GetProcess(url string) (*Process, bool)

	// ListProcesses returns all registered processes
	ListProcesses() map[string]*Process
}

// DefaultProcessTracker implements ProcessTracker
type DefaultProcessTracker struct {
	processes sync.Map
}

// NewProcessTracker creates a new process tracker
func NewProcessTracker() *DefaultProcessTracker {
	return &DefaultProcessTracker{}
}

// RegisterProcess registers a process with a URL
func (t *DefaultProcessTracker) RegisterProcess(url string, process *Process) {
	t.processes.Store(url, process)
}

// UnregisterProcess removes a process registration
func (t *DefaultProcessTracker) UnregisterProcess(url string) {
	t.processes.Delete(url)
}

// GetProcess returns a process by URL
func (t *DefaultProcessTracker) GetProcess(url string) (*Process, bool) {
	if proc, ok := t.processes.Load(url); ok {
		return proc.(*Process), true
	}
	return nil, false
}

// ListProcesses returns all registered processes
func (t *DefaultProcessTracker) ListProcesses() map[string]*Process {
	result := make(map[string]*Process)
	t.processes.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*Process)
		return true
	})
	return result
}

// SystemProcessFinder finds FFmpeg processes in the system
type SystemProcessFinder interface {
	// FindFFmpegProcesses finds all FFmpeg processes in the system
	FindFFmpegProcesses() ([]ProcessInfo, error)

	// IsProcessRunning checks if a process is running
	IsProcessRunning(pid int) bool

	// TerminateProcess terminates a process by PID
	TerminateProcess(pid int) error
}

// DefaultSystemProcessFinder implements SystemProcessFinder
type DefaultSystemProcessFinder struct{}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// FindFFmpegProcesses finds all FFmpeg processes in the system
func (f *DefaultSystemProcessFinder) FindFFmpegProcesses() ([]ProcessInfo, error) {
	if IsWindows() {
		return f.findFFmpegProcessesWindows()
	}
	return f.findFFmpegProcessesUnix()
}

// findFFmpegProcessesUnix finds FFmpeg processes on Unix systems
func (f *DefaultSystemProcessFinder) findFFmpegProcessesUnix() ([]ProcessInfo, error) {
	cmd := exec.Command("pgrep", "ffmpeg")
	output, err := cmd.Output()
	if err != nil {
		// No processes found is not an error
		if strings.Contains(err.Error(), "exit status 1") {
			return []ProcessInfo{}, nil
		}
		return nil, fmt.Errorf("error finding FFmpeg processes: %w", err)
	}

	var processes []ProcessInfo
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			var pid int
			if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
				processes = append(processes, ProcessInfo{
					PID:  pid,
					Name: "ffmpeg",
				})
			}
		}
	}

	return processes, nil
}

// findFFmpegProcessesWindows finds FFmpeg processes on Windows systems
func (f *DefaultSystemProcessFinder) findFFmpegProcessesWindows() ([]ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq ffmpeg.exe", "/NH", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error finding FFmpeg processes: %w", err)
	}

	var processes []ProcessInfo
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "ffmpeg.exe") {
			fields := strings.Split(line, ",")
			if len(fields) >= 2 {
				// Remove quotes and convert to PID
				pidStr := strings.Trim(fields[1], "\" \r\n")
				var pid int
				if _, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil {
					processes = append(processes, ProcessInfo{
						PID:  pid,
						Name: "ffmpeg.exe",
					})
				}
			}
		}
	}

	return processes, nil
}

// IsProcessRunning checks if a process is running
func (f *DefaultSystemProcessFinder) IsProcessRunning(pid int) bool {
	if IsWindows() {
		return f.isProcessRunningWindows(pid)
	}
	return f.isProcessRunningUnix(pid)
}

// isProcessRunningUnix checks if a process is running on Unix systems
func (f *DefaultSystemProcessFinder) isProcessRunningUnix(pid int) bool {
	cmd := exec.Command("kill", "-0", fmt.Sprint(pid))
	return cmd.Run() == nil
}

// isProcessRunningWindows checks if a process is running on Windows systems
func (f *DefaultSystemProcessFinder) isProcessRunningWindows(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", "PID eq "+fmt.Sprint(pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), fmt.Sprint(pid))
}

// TerminateProcess terminates a process by PID
func (f *DefaultSystemProcessFinder) TerminateProcess(pid int) error {
	if IsWindows() {
		return f.terminateProcessWindows(pid)
	}
	return f.terminateProcessUnix(pid)
}

// terminateProcessUnix terminates a process on Unix systems
func (f *DefaultSystemProcessFinder) terminateProcessUnix(pid int) error {
	cmd := exec.Command("kill", "-9", fmt.Sprint(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

// terminateProcessWindows terminates a process on Windows systems
func (f *DefaultSystemProcessFinder) terminateProcessWindows(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprint(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to terminate process %d: %w", pid, err)
	}
	return nil
}

// Monitor monitors and manages FFmpeg processes
type Monitor struct {
	tracker       ProcessTracker
	processFinder SystemProcessFinder
	interval      time.Duration
	running       atomic.Bool
	stopCh        chan struct{}
	urls          []string
	urlsMu        sync.RWMutex
}

// MonitorOptions contains options for creating a monitor
type MonitorOptions struct {
	Interval       time.Duration
	ProcessFinder  SystemProcessFinder
	ProcessTracker ProcessTracker
}

// NewMonitor creates a new FFmpeg process monitor
func NewMonitor(opts MonitorOptions) *Monitor {
	if opts.Interval == 0 {
		opts.Interval = 30 * time.Second
	}

	if opts.ProcessFinder == nil {
		opts.ProcessFinder = &DefaultSystemProcessFinder{}
	}

	if opts.ProcessTracker == nil {
		opts.ProcessTracker = NewProcessTracker()
	}

	return &Monitor{
		tracker:       opts.ProcessTracker,
		processFinder: opts.ProcessFinder,
		interval:      opts.Interval,
		stopCh:        make(chan struct{}),
		urls:          make([]string, 0),
	}
}

// Start starts the monitor
func (m *Monitor) Start() {
	if m.running.Load() {
		return
	}

	m.running.Store(true)
	go m.monitorLoop()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if !m.running.Load() {
		return
	}

	m.running.Store(false)
	close(m.stopCh)
	m.stopCh = make(chan struct{})
}

// IsRunning returns whether the monitor is running
func (m *Monitor) IsRunning() bool {
	return m.running.Load()
}

// UpdateURLs updates the list of known URLs
func (m *Monitor) UpdateURLs(urls []string) {
	m.urlsMu.Lock()
	defer m.urlsMu.Unlock()

	m.urls = make([]string, len(urls))
	copy(m.urls, urls)
}

// monitorLoop periodically checks FFmpeg processes
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if err := m.checkProcesses(); err != nil {
				fmt.Printf("Error checking FFmpeg processes: %v\n", err)
			}
		}
	}
}

// checkProcesses checks for orphaned FFmpeg processes
func (m *Monitor) checkProcesses() error {
	// Get list of configured URLs
	m.urlsMu.RLock()
	configuredURLs := make(map[string]bool, len(m.urls))
	for _, url := range m.urls {
		configuredURLs[url] = true
	}
	m.urlsMu.RUnlock()

	// Check tracked processes against configuration
	processes := m.tracker.ListProcesses()
	for url, proc := range processes {
		// If URL is not in configuration, stop the process
		if !configuredURLs[url] {
			fmt.Printf("Stopping orphaned FFmpeg process for URL %s\n", url)
			if err := proc.Stop(); err != nil {
				fmt.Printf("Error stopping FFmpeg process for URL %s: %v\n", url, err)
				// Continue with unregistering even if there was an error stopping
			}
			m.tracker.UnregisterProcess(url)
		}
	}

	// Check for orphaned system processes
	if err := m.checkOrphanedSystemProcesses(); err != nil {
		return fmt.Errorf("error checking orphaned system processes: %w", err)
	}

	return nil
}

// checkOrphanedSystemProcesses checks for orphaned FFmpeg processes at the system level
func (m *Monitor) checkOrphanedSystemProcesses() error {
	// Get all FFmpeg processes from the system
	systemProcesses, err := m.processFinder.FindFFmpegProcesses()
	if err != nil {
		return err
	}

	// Get tracked processes
	trackedProcesses := m.tracker.ListProcesses()

	// Track PIDs of processes we know about
	knownPIDs := make(map[int]bool)
	for _, proc := range trackedProcesses {
		if proc.cmd != nil && proc.cmd.Process() != nil {
			knownPIDs[proc.cmd.Process().Pid] = true
		}
	}

	// Terminate orphaned processes
	for _, proc := range systemProcesses {
		if !knownPIDs[proc.PID] {
			// Check if the process is actually running
			if m.processFinder.IsProcessRunning(proc.PID) {
				fmt.Printf("Terminating orphaned FFmpeg process with PID %d\n", proc.PID)
				if err := m.processFinder.TerminateProcess(proc.PID); err != nil {
					fmt.Printf("Error terminating process %d: %v\n", proc.PID, err)
				}
			}
		}
	}

	return nil
}

// RegisterProcess registers a process with the monitor
func (m *Monitor) RegisterProcess(url string, process *Process) {
	m.tracker.RegisterProcess(url, process)

	// Add URL to the known URLs
	m.urlsMu.Lock()
	found := false
	for _, u := range m.urls {
		if u == url {
			found = true
			break
		}
	}
	if !found {
		m.urls = append(m.urls, url)
	}
	m.urlsMu.Unlock()
}

// UnregisterProcess removes a process registration
func (m *Monitor) UnregisterProcess(url string) {
	m.tracker.UnregisterProcess(url)

	// Remove URL from the known URLs
	m.urlsMu.Lock()
	for i, u := range m.urls {
		if u == url {
			m.urls = append(m.urls[:i], m.urls[i+1:]...)
			break
		}
	}
	m.urlsMu.Unlock()
}
