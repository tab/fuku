package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/terminal"
)

// startupState is where one service is in a detached start
type startupState int

// startupState values
const (
	stateWaiting startupState = iota
	stateStarting
	stateReady
	stateFailed
)

// startupDone ends the startup view after its final frame
type startupDone struct{}

// Startup renders a detached start the way docker compose up -d does: one line per service, updated in place
type Startup struct {
	options Options
	theme   func() terminal.Theme
	stdout  io.Writer
	program *tea.Program
	done    chan struct{}
}

// NewStartup creates the startup view of a detached run
func NewStartup(options Options, theme func() terminal.Theme, stdout io.Writer) *Startup {
	return &Startup{options: options, theme: theme, stdout: stdout}
}

// Open starts the view inline; it reads the terminal, so the replies to its queries never reach the shell
func (s *Startup) Open() {
	opened := time.Now()

	s.program = tea.NewProgram(
		newStartupModel(s.options.Profile, s.theme(), opened),
		tea.WithoutSignalHandler(),
		tea.WithOutput(s.stdout),
	)
	s.done = make(chan struct{})

	go func() {
		defer close(s.done)

		//nolint:errcheck // the view is best effort; the command reports the outcome
		s.program.Run()
	}()
}

// Show applies one record to the view
func (s *Startup) Show(record detach.Record) {
	s.program.Send(record)
}

// Close renders the final frame, leaves it on screen and waits for the view to end
func (s *Startup) Close() {
	s.program.Send(startupDone{})
	<-s.done
}

// startupService is one line of the view
type startupService struct {
	name     string
	state    startupState
	duration time.Duration
}

// startupModel is the Bubble Tea model of the startup view
type startupModel struct {
	profile  string
	theme    terminal.Theme
	opened   time.Time
	services []*startupService
	spinner  spinner.Model
}

// newStartupModel creates the model with the spinner of the services view
func newStartupModel(profile string, theme terminal.Theme, opened time.Time) startupModel {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = terminal.SpinnerStyle

	return startupModel{profile: profile, theme: theme, opened: opened, spinner: s}
}

// Init starts the spinner
func (m startupModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update applies a record, advances the spinner or ends the view
func (m startupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detach.Record:
		m.apply(msg)

		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, interrupt
		}
	case startupDone:
		return m, tea.Quit
	}

	return m, nil
}

// apply moves the service a record names to its new state
func (m *startupModel) apply(record detach.Record) {
	switch record.Kind {
	case detach.KindProfile:
		for _, name := range record.Services {
			m.services = append(m.services, &startupService{name: name})
		}
	case detach.KindStarting:
		m.set(record.Service, stateStarting, 0)
	case detach.KindReady:
		m.set(record.Service, stateReady, record.Duration)
	case detach.KindFailed:
		m.set(record.Service, stateFailed, time.Since(m.opened))
	default:
		// no-op: the summary reports the API and the running instance
	}
}

// set updates one service by name
func (m *startupModel) set(name string, state startupState, duration time.Duration) {
	for _, svc := range m.services {
		if svc.name == name {
			svc.state = state
			svc.duration = duration
		}
	}
}

// View renders the title with the settled count and one line per service
func (m startupModel) View() tea.View {
	var b strings.Builder

	fmt.Fprintf(&b, "%s run %s %d/%d\n", terminal.BoldStyle.Render("[+]"), m.profile, m.ready(), len(m.services))

	width := 0
	for _, svc := range m.services {
		width = max(width, len(svc.name))
	}

	elapsed := time.Since(m.opened)

	for _, svc := range m.services {
		glyph, label, duration := m.row(svc, elapsed)
		fmt.Fprintf(&b, " %s %-*s  %s  %4.1fs\n", glyph, width, svc.name, label, duration.Seconds())
	}

	return tea.NewView(b.String())
}

// row returns the glyph, the styled state and the time of one service
func (m startupModel) row(svc *startupService, elapsed time.Duration) (string, string, time.Duration) {
	label := func(style lipgloss.Style, text string) string {
		return style.Render(fmt.Sprintf("%-8s", text))
	}

	switch svc.state {
	case stateReady:
		return m.theme.StatusRunningStyle.Render("✔"), label(m.theme.StatusRunningStyle, "Ready"), svc.duration
	case stateFailed:
		return m.theme.StatusFailedStyle.Render("✗"), label(m.theme.StatusFailedStyle, "Failed"), svc.duration
	case stateStarting:
		return m.spinner.View(), label(m.theme.StatusStartingStyle, "Starting"), elapsed
	default:
		return m.spinner.View(), label(m.theme.StatusPendingStyle, "Waiting"), elapsed
	}
}

// ready counts the services that are ready
func (m startupModel) ready() int {
	count := 0

	for _, svc := range m.services {
		if svc.state == stateReady {
			count++
		}
	}

	return count
}

// interrupt turns Ctrl-C, a key press in raw mode, back into the SIGINT the parent stops on
func interrupt() tea.Msg {
	//nolint:errcheck // signalling the own process does not fail
	syscall.Kill(os.Getpid(), syscall.SIGINT)

	return nil
}
