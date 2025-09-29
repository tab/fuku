//go:generate mockgen -source=ui.go -destination=ui_mock.go -package=ui
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fuku/internal/app/colors"
	"fuku/internal/app/tracker"
)

// TrackerResult is a type alias for tracker.Result to avoid import issues
type TrackerResult = tracker.Result

// Progress manages startup progress visualization
type Progress interface {
	SetProgress(currentLayer, totalLayers int, step string)
	ShowLayer(layerNum int, layerServices []string)
	UpdateLayer(layerNum int, layerServices []string)
	IsReady() bool
	ShowSummary()
}

// Mode handles display modes and phases
type Mode interface {
	Phase(name string)
	IsBootstrap() bool
	SwitchToLogs()
	BufferLog(line string)
}

// Output handles final output and errors
type Output interface {
	ShowError(serviceName string, err error)
	ShowSuccess()
	ProgressBar(current, total int, width int) string
}

// Display combines all display interfaces for the main implementation
type Display interface {
	Progress
	Mode
	Output
	// Service management
	Add(name string)
	Update(name string, status tracker.Status)
	Error(name string, err error)
	// Testing methods
	GetTotalServices() int
	GetServicesStarted() int
	GetCurrentLayer() int
	GetTotalLayers() int
	GetCurrentStep() string
	GetAnalytics() *Analytics
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

// display manages the enhanced CLI display
type display struct {
	tracker    tracker.Tracker
	progress   *ProgressInfo
	analytics  *Analytics
	mu         sync.RWMutex
	isActive   bool
	lastUpdate time.Time
	mode       DisplayMode
	logBuffer  *LogBuffer
	services   map[string]TrackerResult
	startTimes map[string]time.Time
}

// NewDisplay creates a new enhanced display manager
func NewDisplay(tracker tracker.Tracker) Display {
	return &display{
		tracker:    tracker,
		progress:   &ProgressInfo{StartTime: time.Now()},
		analytics:  &Analytics{LayerDurations: make(map[int]time.Duration)},
		isActive:   true,
		mode:       ModeBootstrap,
		logBuffer:  &LogBuffer{},
		services:   make(map[string]TrackerResult),
		startTimes: make(map[string]time.Time),
	}
}

// GetStatusSymbol returns colored status symbols for clean display
func GetStatusSymbol(s tracker.Status) string {
	switch s {
	case tracker.StatusPending:
		return colors.StatusPendingColor(colors.StatusPending)
	case tracker.StatusStarting:
		return colors.StatusRunningColor(colors.StatusRunning)
	case tracker.StatusRunning:
		return colors.StatusSuccessColor(colors.StatusSuccess)
	case tracker.StatusFailed:
		return colors.StatusFailedColor(colors.StatusFailed)
	case tracker.StatusStopped:
		return colors.StatusStoppedColor(colors.StatusStopped)
	default:
		return colors.StatusPendingColor(colors.StatusPending)
	}
}

// Add adds a service to track
func (ed *display) Add(name string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	result := ed.tracker.Add(name)
	ed.services[name] = result
	ed.startTimes[name] = time.Now()
	ed.progress.TotalServices++
}

// Update updates a service status
func (ed *display) Update(name string, status tracker.Status) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if result, exists := ed.services[name]; exists {
		oldStatus := result.GetStatus()
		result.SetStatus(status)
		duration := time.Since(ed.startTimes[name])

		if oldStatus != tracker.StatusRunning && status == tracker.StatusRunning {
			ed.progress.ServicesStarted++

			if ed.analytics.FastestTime == 0 || duration < ed.analytics.FastestTime {
				ed.analytics.FastestTime = duration
				ed.analytics.FastestService = name
			}
			if duration > ed.analytics.SlowestTime {
				ed.analytics.SlowestTime = duration
				ed.analytics.SlowestService = name
			}
		}
	}
	ed.lastUpdate = time.Now()
}

// Error sets an error for a service
func (ed *display) Error(name string, err error) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if result, exists := ed.services[name]; exists {
		result.SetError(err)
	}
}

// UpdateProgress updates the overall progress
func (ed *display) SetProgress(currentLayer, totalLayers int, step string) {
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
func (ed *display) SwitchToLogs() {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.mode = ModeLogs
}

// BufferLog adds a log line to the buffer during bootstrap mode
func (ed *display) BufferLog(line string) {
	if ed.IsBootstrap() {
		ed.logBuffer.Add(line)
	} else {
		fmt.Println(line)
	}
}

// IsBootstrapMode returns true if still in bootstrap mode
func (ed *display) IsBootstrap() bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.mode == ModeBootstrap
}

// Phase shows the current phase with colored formatting
func (ed *display) Phase(name string) {
	var details string
	switch name {
	case "Discovery":
		name = "Discovery"
		details = "Scanning services and profiles"
	case "Planning":
		name = "Planning"
		details = "Resolving dependencies and layers"
	case "Execution":
		name = "Execution"
		details = "Starting services in order"
	default:
		details = "Processing"
	}

	fmt.Printf("%s\n", colors.FormatPhase(name, details))
}

// GenerateProgressBar creates a visual progress bar
func (ed *display) ProgressBar(current, total int, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}

	progress := float64(current) / float64(total)
	filled := int(progress * float64(width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := int(progress * 100)

	return fmt.Sprintf("%s %d/%d (%d%%)", bar, current, total, percentage)
}

// ShowLayer shows progress for current layer (initial display)
func (ed *display) ShowLayer(layerNum int, layerServices []string) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	fmt.Printf("   %s layer %d (%d services):\n",
		colors.Muted("starting"), layerNum, len(layerServices))

	for i, serviceName := range layerServices {
		result := ed.services[serviceName]
		status := GetStatusSymbol(result.GetStatus())

		prefix := colors.TreeBranch
		if i == len(layerServices)-1 {
			prefix = colors.TreeLast
		}

		fmt.Printf("   %s %s %s\n", colors.Muted(prefix), status, colors.Primary(serviceName))
	}
	fmt.Println()
}

// UpdateLayer updates only the service status lines without redrawing the header
func (ed *display) UpdateLayer(layerNum int, layerServices []string) {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	// Move cursor up to overwrite the service lines (number of services + 1 empty line)
	fmt.Printf("\033[%dA", len(layerServices)+1)

	// Clear from cursor to end of screen and redraw only the service lines
	fmt.Printf("\033[J")

	for i, serviceName := range layerServices {
		result := ed.services[serviceName]
		status := GetStatusSymbol(result.GetStatus())

		prefix := colors.TreeBranch
		if i == len(layerServices)-1 {
			prefix = colors.TreeLast
		}

		fmt.Printf("   %s %s %s\n", colors.Muted(prefix), status, colors.Primary(serviceName))
	}
	fmt.Println()
}

// IsReady checks if all services have ✅ status
func (ed *display) IsReady() bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	for _, result := range ed.services {
		if result.GetStatus() != tracker.StatusRunning {
			return false
		}
	}
	return len(ed.services) > 0
}

// ShowSummary shows a compact summary during bootstrap
func (ed *display) ShowSummary() {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	if ed.mode != ModeBootstrap {
		return
	}

	running := 0
	failed := 0
	for _, result := range ed.services {
		switch result.GetStatus() {
		case tracker.StatusRunning:
			running++
		case tracker.StatusFailed:
			failed++
		}
	}

	fmt.Printf("%s %s: %s",
		colors.StatusSuccessColor(colors.StatusSuccess),
		colors.Success("Running"),
		colors.Primary(fmt.Sprintf("%d/%d", running, ed.progress.TotalServices)))

	if failed > 0 {
		fmt.Printf("  %s %s: %s",
			colors.StatusFailedColor(colors.StatusFailed),
			colors.Error("Failed"),
			colors.Error(fmt.Sprintf("%d", failed)))
	}

	fmt.Printf("  %s %s: %s\n",
		colors.Muted("|"),
		colors.Info("Time"),
		colors.Muted(time.Since(ed.progress.StartTime).Truncate(time.Millisecond).String()))
}

// DisplayError shows enhanced error information
func (ed *display) ShowError(serviceName string, err error) {
	fmt.Printf("\n%s %s\n",
		colors.StatusFailedColor(colors.StatusFailed),
		colors.Error("Service Failure Detected"))

	fmt.Printf("%s\n", colors.Muted("┌─────────────────────────────────────────────────────────────────────┐"))
	fmt.Printf("%s %s %-59s %s\n",
		colors.Muted("│"),
		colors.StatusFailedColor(colors.StatusFailed),
		colors.Error(serviceName),
		colors.Muted("│"))

	fmt.Printf("%s    %s %s %-50s %s\n",
		colors.Muted("│"),
		colors.Error("Error:"),
		colors.Muted(""),
		truncateString(err.Error(), 50),
		colors.Muted("│"))

	// Add contextual suggestions based on error type
	suggestion := getSuggestionForError(err.Error())
	if suggestion != "" {
		fmt.Printf("%s    %s %-44s %s\n",
			colors.Muted("│"),
			colors.Warning("Suggestion:"),
			colors.Muted(truncateString(suggestion, 44)),
			colors.Muted("│"))
	}

	fmt.Printf("%s\n\n", colors.Muted("└─────────────────────────────────────────────────────────────────────┘"))
}

// DisplaySuccess shows the success summary with analytics
func (ed *display) ShowSuccess() {
	ed.mu.Lock()
	ed.analytics.TotalDuration = time.Since(ed.progress.StartTime)
	ed.analytics.ServicesCount = len(ed.services)
	ed.mu.Unlock()

	fmt.Printf("\n%s\n", colors.Title("Summary"))

	ed.mu.RLock()
	for i := 0; i < ed.progress.TotalLayers; i++ {
		if duration, exists := ed.analytics.LayerDurations[i]; exists {
			fmt.Printf("  %s %s %d: %s\n",
				colors.Muted(colors.TreeBranch),
				colors.Info("Layer"),
				i,
				colors.Primary(duration.Truncate(time.Millisecond).String()))
		}
	}

	if ed.analytics.FastestService != "" {
		fmt.Printf("  %s %s %s (%s)\n",
			colors.Muted(colors.TreeBranch),
			colors.Success("Fastest:"),
			colors.Primary(ed.analytics.FastestService),
			colors.Muted(ed.analytics.FastestTime.Truncate(time.Millisecond).String()))
	}

	if ed.analytics.SlowestService != "" {
		fmt.Printf("  %s %s %s (%s)\n",
			colors.Muted(colors.TreeBranch),
			colors.Warning("Slowest:"),
			colors.Primary(ed.analytics.SlowestService),
			colors.Muted(ed.analytics.SlowestTime.Truncate(time.Millisecond).String()))
	}
	ed.mu.RUnlock()

	fmt.Printf("  %s %s %s\n",
		colors.Muted(colors.TreeLast),
		colors.Info("Total:"),
		colors.Primary(ed.analytics.TotalDuration.Truncate(time.Millisecond).String()))

	ed.SwitchToLogs()
	fmt.Printf("%s\n", colors.Muted("────────────────────────────────────────────────────────────────"))

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

// Testing methods implementation

// GetTotalServices returns the total number of services
func (ed *display) GetTotalServices() int {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.progress.TotalServices
}

// GetServicesStarted returns the number of services that have started
func (ed *display) GetServicesStarted() int {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.progress.ServicesStarted
}

// GetCurrentLayer returns the current layer for testing
func (ed *display) GetCurrentLayer() int {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.progress.CurrentLayer
}

// GetTotalLayers returns the total layers for testing
func (ed *display) GetTotalLayers() int {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.progress.TotalLayers
}

// GetCurrentStep returns the current step for testing
func (ed *display) GetCurrentStep() string {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.progress.CurrentStep
}

// GetAnalytics returns analytics for testing
func (ed *display) GetAnalytics() *Analytics {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.analytics
}
