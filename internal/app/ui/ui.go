package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fuku/internal/app/results"
)

// ServiceDisplayInfo holds display information for a service
type ServiceDisplayInfo struct {
	Name        string
	Status      results.ServiceStatus
	Error       error
	StartTime   time.Time
	Duration    time.Duration
	Port        string
	CPU         string
	Memory      string
	LastMessage string
}

// ProgressInfo tracks overall progress
type ProgressInfo struct {
	CurrentLayer    int
	TotalLayers     int
	CurrentStep     string
	ServicesStarted int
	TotalServices   int
	StartTime       time.Time
	LayerStartTime  time.Time
}

// Analytics holds startup performance data
type Analytics struct {
	TotalDuration  time.Duration
	LayerDurations map[int]time.Duration
	FastestService string
	SlowestService string
	FastestTime    time.Duration
	SlowestTime    time.Duration
	ServicesCount  int
}

// DisplayMode controls when to show UI vs logs
type DisplayMode int

const (
	ModeBootstrap DisplayMode = iota
	ModeLogs
)

// LogBuffer holds buffered logs during bootstrap
type LogBuffer struct {
	lines []string
	mu    sync.Mutex
}

func (lb *LogBuffer) Add(line string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.lines = append(lb.lines, line)
}

func (lb *LogBuffer) Flush() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for _, line := range lb.lines {
		fmt.Println(line)
	}
	lb.lines = nil
}

// Display manages the enhanced CLI display
type Display struct {
	services   map[string]*ServiceDisplayInfo
	progress   *ProgressInfo
	analytics  *Analytics
	mu         sync.RWMutex
	isActive   bool
	lastUpdate time.Time
	mode       DisplayMode
	logBuffer  *LogBuffer
}

// NewDisplay creates a new enhanced display manager
func NewDisplay() *Display {
	return &Display{
		services:  make(map[string]*ServiceDisplayInfo),
		progress:  &ProgressInfo{StartTime: time.Now()},
		analytics: &Analytics{LayerDurations: make(map[int]time.Duration)},
		isActive:  true,
		mode:      ModeBootstrap,
		logBuffer: &LogBuffer{},
	}
}


// GetStatusEmoji returns just the emoji for compact display
func GetStatusEmoji(s results.ServiceStatus) string {
	switch s {
	case results.StatusPending:
		return "⏳"
	case results.StatusStarting:
		return "🚀"
	case results.StatusRunning:
		return "✅"
	case results.StatusFailed:
		return "❌"
	case results.StatusStopped:
		return "⏹️"
	default:
		return "❓"
	}
}

// AddService adds a service to track
func (ed *Display) AddService(name string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.services[name] = &ServiceDisplayInfo{
		Name:      name,
		Status:    results.StatusPending,
		StartTime: time.Now(),
	}
	ed.progress.TotalServices++
}

// UpdateServiceStatus updates a service status
func (ed *Display) UpdateServiceStatus(name string, status results.ServiceStatus) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if service, exists := ed.services[name]; exists {
		oldStatus := service.Status
		service.Status = status
		service.Duration = time.Since(service.StartTime)

		if oldStatus != results.StatusRunning && status == results.StatusRunning {
			ed.progress.ServicesStarted++

			if ed.analytics.FastestTime == 0 || service.Duration < ed.analytics.FastestTime {
				ed.analytics.FastestTime = service.Duration
				ed.analytics.FastestService = name
			}
			if service.Duration > ed.analytics.SlowestTime {
				ed.analytics.SlowestTime = service.Duration
				ed.analytics.SlowestService = name
			}
		}
	}
	ed.lastUpdate = time.Now()
}

// SetServiceError sets an error for a service
func (ed *Display) SetServiceError(name string, err error) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if service, exists := ed.services[name]; exists {
		service.Error = err
	}
}

// UpdateProgress updates the overall progress
func (ed *Display) UpdateProgress(currentLayer, totalLayers int, step string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if currentLayer != ed.progress.CurrentLayer {
		if ed.progress.CurrentLayer > 0 {
			layerDuration := time.Since(ed.progress.LayerStartTime)
			ed.analytics.LayerDurations[ed.progress.CurrentLayer-1] = layerDuration
		}
		ed.progress.LayerStartTime = time.Now()
	}

	ed.progress.CurrentLayer = currentLayer
	ed.progress.TotalLayers = totalLayers
	ed.progress.CurrentStep = step
	ed.lastUpdate = time.Now()
}

// SwitchToLogsMode switches from bootstrap UI to logs mode
func (ed *Display) SwitchToLogsMode() {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.mode = ModeLogs
}

// BufferLog adds a log line to the buffer during bootstrap mode
func (ed *Display) BufferLog(line string) {
	if ed.IsBootstrapMode() {
		ed.logBuffer.Add(line)
	} else {
		fmt.Println(line)
	}
}

// IsBootstrapMode returns true if still in bootstrap mode
func (ed *Display) IsBootstrapMode() bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.mode == ModeBootstrap
}

// Phase shows the current phase
func (ed *Display) Phase(phase, description string) {
	fmt.Printf("Phase %s: %s\n", phase, description)
}

// GenerateProgressBar creates a visual progress bar
func (ed *Display) GenerateProgressBar(current, total int, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}

	progress := float64(current) / float64(total)
	filled := int(progress * float64(width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := int(progress * 100)

	return fmt.Sprintf("%s %d/%d (%d%%)", bar, current, total, percentage)
}

// DisplayLayerProgress shows progress for current layer (initial display)
func (ed *Display) DisplayLayerProgress(layerNum int, layerServices []string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	fmt.Printf("  Starting Layer %d (%d services):\n", layerNum, len(layerServices))

	for i, serviceName := range layerServices {
		service := ed.services[serviceName]
		status := GetStatusEmoji(service.Status)

		prefix := "├─"
		if i == len(layerServices)-1 {
			prefix = "└─"
		}

		fmt.Printf("   %s %s %s\n", prefix, status, serviceName)
	}
	fmt.Println()
}

// UpdateLayerProgress updates only the service status lines without redrawing the header
func (ed *Display) UpdateLayerProgress(layerNum int, layerServices []string) {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	// Move cursor up to overwrite the service lines (number of services + 1 empty line)
	fmt.Printf("\033[%dA", len(layerServices)+1)

	// Clear from cursor to end of screen and redraw only the service lines
	fmt.Printf("\033[J")

	for i, serviceName := range layerServices {
		service := ed.services[serviceName]
		status := GetStatusEmoji(service.Status)

		prefix := "├─"
		if i == len(layerServices)-1 {
			prefix = "└─"
		}

		fmt.Printf("   %s %s %s\n", prefix, status, serviceName)
	}
	fmt.Println()
}

// AreAllServicesRunning checks if all services have ✅ status
func (ed *Display) AreAllServicesRunning() bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	for _, service := range ed.services {
		if service.Status != results.StatusRunning {
			return false
		}
	}
	return len(ed.services) > 0
}


// DisplayBootstrapSummary shows a compact summary during bootstrap
func (ed *Display) DisplayBootstrapSummary() {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	if ed.mode != ModeBootstrap {
		return
	}

	running := 0
	failed := 0
	for _, service := range ed.services {
		switch service.Status {
		case results.StatusRunning:
			running++
		case results.StatusFailed:
			failed++
		}
	}

	fmt.Printf("✅ Running: %d/%d", running, ed.progress.TotalServices)
	if failed > 0 {
		fmt.Printf("  ❌ Failed: %d", failed)
	}
	fmt.Printf("  |  ⏱️ Time: %s\n", time.Since(ed.progress.StartTime).Truncate(time.Millisecond))
}

// DisplayError shows enhanced error information
func (ed *Display) DisplayError(serviceName string, err error) {
	fmt.Printf("\n❌ Service Failure Detected:\n")
	fmt.Printf("┌─────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│ 🔴 %-63s │\n", serviceName)
	fmt.Printf("│    📍 Error: %-54s │\n", truncateString(err.Error(), 54))

	// Add contextual suggestions based on error type
	suggestion := getSuggestionForError(err.Error())
	if suggestion != "" {
		fmt.Printf("│    💡 Suggestion: %-48s │\n", truncateString(suggestion, 48))
	}

	fmt.Printf("└─────────────────────────────────────────────────────────────────────┘\n\n")
}

// DisplaySuccess shows the success summary with analytics
func (ed *Display) DisplaySuccess() {
	ed.mu.Lock()
	ed.analytics.TotalDuration = time.Since(ed.progress.StartTime)
	ed.analytics.ServicesCount = len(ed.services)
	ed.mu.Unlock()

	fmt.Printf("\nSummary:\n")

	ed.mu.RLock()
	for i := 0; i < ed.progress.TotalLayers; i++ {
		if duration, exists := ed.analytics.LayerDurations[i]; exists {
			fmt.Printf("  ├─ Layer %d: %s\n", i, duration.Truncate(time.Millisecond))
		}
	}

	if ed.analytics.FastestService != "" {
		fmt.Printf("  ├─ 🏆 Fastest: %s (%s)\n",
			ed.analytics.FastestService,
			ed.analytics.FastestTime.Truncate(time.Millisecond))
	}

	if ed.analytics.SlowestService != "" {
		fmt.Printf("  ├─ 🐌 Slowest: %s (%s)\n",
			ed.analytics.SlowestService,
			ed.analytics.SlowestTime.Truncate(time.Millisecond))
	}
	ed.mu.RUnlock()

	fmt.Printf("  └─ ⏱️ Total: %s\n", ed.analytics.TotalDuration.Truncate(time.Millisecond))

	ed.SwitchToLogsMode()
	fmt.Printf("────────────────────────────────────────────────────────────────\n")

	ed.logBuffer.Flush()
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getSuggestionForError(errorMsg string) string {
	errorLower := strings.ToLower(errorMsg)

	switch {
	case strings.Contains(errorLower, "port") && strings.Contains(errorLower, "already in use"):
		return "Check if another instance is running with 'lsof -ti:PORT'"
	case strings.Contains(errorLower, "permission denied"):
		return "Try running with appropriate permissions"
	case strings.Contains(errorLower, "no such file"):
		return "Verify the service directory and Makefile exist"
	case strings.Contains(errorLower, "connection refused"):
		return "Check if dependent services are running"
	case strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "timed out"):
		return "Service may need more time to start, check logs"
	default:
		return "Check service logs for more details"
	}
}
