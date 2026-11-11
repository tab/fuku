package tui

import (
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/terminal"
)

// MCP console column width constants, the widths of the plain lines
const (
	mcpLabelWidth   = 9
	mcpTargetWidth  = 20
	mcpOutcomeWidth = 9
)

// mcpConsoleDone ends the console view after its final frame, once every call line is printed
type mcpConsoleDone struct{}

// mcpConsolePrinted reports that the line of one call is printed above the view
type mcpConsolePrinted struct{}

// MCPConsole renders the fuku mcp banner in place on a terminal stderr and prints each tool call above it
type MCPConsole struct {
	theme   terminal.Theme
	stderr  io.Writer
	program *tea.Program
	done    chan struct{}
}

// NewMCPConsole creates the live console of fuku mcp
func NewMCPConsole(theme terminal.Theme, stderr io.Writer) *MCPConsole {
	return &MCPConsole{theme: theme, stderr: stderr}
}

// Open starts the view inline on stderr with no input, so it reads no stdin and sends no terminal query
func (c *MCPConsole) Open() {
	c.program = tea.NewProgram(
		mcpConsoleModel{theme: c.theme},
		tea.WithInput(nil),
		tea.WithOutput(c.stderr),
		tea.WithoutSignalHandler(),
	)
	c.done = make(chan struct{})

	go func() {
		defer close(c.done)

		//nolint:errcheck // the console is best effort; the server keeps serving without it
		c.program.Run()
	}()
}

// Banner shows the startup banner
func (c *MCPConsole) Banner(banner mcp.Banner) {
	c.program.Send(banner)
}

// Call prints the line of one tool call above the banner, so it stays in the scrollback
func (c *MCPConsole) Call(call mcp.Call) {
	c.program.Send(call)
}

// Close renders the final frame, leaves it on screen and waits for the view to end
func (c *MCPConsole) Close() {
	c.program.Send(mcpConsoleDone{})
	<-c.done
}

// mcpConsoleModel is the Bubble Tea model of the console: the banner it keeps in place and the lines still printing
type mcpConsoleModel struct {
	theme   terminal.Theme
	banner  mcp.Banner
	pending int
	closing bool
}

// Init has nothing to start
func (m mcpConsoleModel) Init() tea.Cmd {
	return nil
}

// Update applies the banner, prints the line of a call above the view, or ends the view once no line is printing
func (m mcpConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mcp.Banner:
		m.banner = msg

		return m, nil
	case mcp.Call:
		m.pending++

		return m, tea.Sequence(tea.Println(m.line(msg)), printed)
	case mcpConsolePrinted:
		m.pending--
	case mcpConsoleDone:
		m.closing = true
	default:
		return m, nil
	}

	if m.closing && m.pending == 0 {
		return m, tea.Quit
	}

	return m, nil
}

// View renders the banner
func (m mcpConsoleModel) View() tea.View {
	if m.banner.Protocol == "" {
		return tea.NewView("")
	}

	var b strings.Builder

	label := func(name string) string {
		return m.theme.PanelMutedStyle.Render(terminal.PadRight(name, mcpLabelWidth))
	}

	fmt.Fprintf(&b, "%s fuku mcp · stdio · protocol %s\n", terminal.BoldStyle.Render("[+]"), m.banner.Protocol)
	fmt.Fprintf(&b, " %s %s · %s\n", label("project"), m.banner.Project, m.banner.Fingerprint)
	fmt.Fprintf(&b, " %s %s · %s\n", label("instance"), m.state(), m.banner.Detail)
	fmt.Fprintf(&b, " %s %s\n", label("tools"), strings.Join(m.banner.Tools, " "))
	b.WriteString("Serving MCP · close stdin or Ctrl+C to stop\n")

	return tea.NewView(b.String())
}

// state renders the instance state in its status color
func (m mcpConsoleModel) state() string {
	switch m.banner.State {
	case mcp.InstanceStateRunning:
		return m.theme.StatusRunningStyle.Render(string(m.banner.State))
	case mcp.InstanceStateNotRunning:
		return m.theme.StatusStoppedStyle.Render(string(m.banner.State))
	default:
		return m.theme.StatusStartingStyle.Render(string(m.banner.State))
	}
}

// line renders one call: the glyph, the target, the outcome or the failure, and the duration
func (m mcpConsoleModel) line(call mcp.Call) string {
	target := terminal.PadRight(call.Target, mcpTargetWidth) + " "

	if call.Failure != "" {
		blank := strings.Repeat(" ", mcpOutcomeWidth+1)

		return fmt.Sprintf(" %s %s%s%s · %s", m.theme.StatusFailedStyle.Render("✘"), target, blank, m.theme.ErrorStyle.Render(call.Failure), call.Elapsed())
	}

	outcome := terminal.PadRight(call.Outcome, mcpOutcomeWidth) + " "

	return fmt.Sprintf(" %s %s%s%s", m.theme.StatusRunningStyle.Render("✔"), target, outcome, m.theme.PanelMutedStyle.Render(call.Elapsed()))
}

// printed reports that the line before it in the sequence is printed
func printed() tea.Msg {
	return mcpConsolePrinted{}
}
