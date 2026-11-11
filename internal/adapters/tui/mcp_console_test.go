package tui

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/mcp"
	"fuku/internal/adapters/terminal"
)

func Test_MCPConsole_Run(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	require.NoError(t, err)

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinReader, stdoutWriter

	t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })

	t.Setenv("TERM", "xterm-ghostty")

	_, err = stdinWriter.WriteString("a protocol message\n")
	require.NoError(t, err)
	require.NoError(t, stdinWriter.Close())

	theme := terminal.NewTheme(terminal.AppearanceDark)
	banner := mcp.Banner{
		Protocol:    "2025-06-18",
		Project:     "/work/shop",
		Fingerprint: "3f2a9c1d",
		State:       mcp.InstanceStateNotRunning,
		Detail:      "call run to start",
		Tools:       []string{"status"},
	}
	call := mcp.Call{Target: "doctor", Duration: 12 * time.Millisecond}

	var stderr bytes.Buffer

	console := NewMCPConsole(theme, &stderr)
	console.Open()
	console.Banner(banner)
	console.Call(call)
	console.Close()

	require.NoError(t, stdoutWriter.Close())

	written, err := io.ReadAll(stdoutReader)
	require.NoError(t, err)

	unread, err := io.ReadAll(stdinReader)
	require.NoError(t, err)

	assert.Empty(t, written)
	assert.Equal(t, "a protocol message\n", string(unread))
	assert.NotContains(t, stderr.String(), ansi.RequestModeSynchronizedOutput)
	assert.Contains(t, stderr.String(), ansi.ShowCursor)
	assert.Contains(t, ansi.Strip(stderr.String()), "doctor")

	select {
	case <-console.done:
	default:
		t.Fatal("the console is still running after Close")
	}
}

func Test_mcpConsoleModel_Init(t *testing.T) {
	assert.Nil(t, mcpConsoleModel{}.Init())
}

func Test_mcpConsoleModel_Update(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	banner := mcp.Banner{
		Protocol:    "2025-06-18",
		Project:     "/work/shop",
		Fingerprint: "3f2a9c1d",
		State:       mcp.InstanceStateRunning,
		Detail:      "profile backend · pid 4242",
		Tools:       []string{"config", "doctor", "logs", "run", "service", "status", "stop"},
	}

	tests := []struct {
		name     string
		messages []tea.Msg
		expected string
	}{
		{
			name:     "renders the banner",
			messages: []tea.Msg{banner},
			expected: strings.Join([]string{
				"[+] fuku mcp · stdio · protocol 2025-06-18",
				" project   /work/shop · 3f2a9c1d",
				" instance  running · profile backend · pid 4242",
				" tools     config doctor logs run service status stop",
				"Serving MCP · close stdin or Ctrl+C to stop",
			}, "\n") + "\n",
		},
		{
			name: "keeps calls out of the view",
			messages: []tea.Msg{
				banner,
				mcp.Call{Target: "status", Duration: time.Millisecond},
			},
			expected: strings.Join([]string{
				"[+] fuku mcp · stdio · protocol 2025-06-18",
				" project   /work/shop · 3f2a9c1d",
				" instance  running · profile backend · pid 4242",
				" tools     config doctor logs run service status stop",
				"Serving MCP · close stdin or Ctrl+C to stop",
			}, "\n") + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current tea.Model = mcpConsoleModel{theme: theme}

			for _, message := range tt.messages {
				current, _ = current.Update(message)
			}

			assert.Equal(t, tt.expected, ansi.Strip(current.View().Content))
		})
	}
}

func Test_mcpConsoleModel_Update_Call(t *testing.T) {
	model := mcpConsoleModel{theme: terminal.NewTheme(terminal.AppearanceDark)}
	call := mcp.Call{Target: "service api restart", Outcome: "running", Duration: 2140 * time.Millisecond}

	current, cmd := model.Update(call)

	require.NotNil(t, cmd)

	steps := reflect.ValueOf(cmd())
	require.Equal(t, 2, steps.Len())

	printLine, ok := reflect.TypeAssert[tea.Cmd](steps.Index(0))
	require.True(t, ok)

	done, ok := reflect.TypeAssert[tea.Cmd](steps.Index(1))
	require.True(t, ok)

	assert.Equal(t, tea.Println(model.line(call))(), printLine())
	assert.Equal(t, mcpConsolePrinted{}, done())
	assert.Equal(t, 1, current.(mcpConsoleModel).pending)
}

func Test_mcpConsoleModel_Update_Done(t *testing.T) {
	_, cmd := mcpConsoleModel{}.Update(mcpConsoleDone{})

	require.NotNil(t, cmd)
	assert.Equal(t, tea.QuitMsg{}, cmd())
}

func Test_mcpConsoleModel_Update_Closing(t *testing.T) {
	tests := []struct {
		name    string
		model   mcpConsoleModel
		message tea.Msg
		quits   bool
	}{
		{
			name:    "done while a line is printing waits",
			model:   mcpConsoleModel{pending: 1},
			message: mcpConsoleDone{},
		},
		{
			name:    "the last printed line after done quits",
			model:   mcpConsoleModel{pending: 1, closing: true},
			message: mcpConsolePrinted{},
			quits:   true,
		},
		{
			name:    "a printed line before done keeps the view",
			model:   mcpConsoleModel{pending: 1},
			message: mcpConsolePrinted{},
		},
		{
			name:    "a printed line while another prints after done waits",
			model:   mcpConsoleModel{pending: 2, closing: true},
			message: mcpConsolePrinted{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := tt.model.Update(tt.message)

			assert.Equal(t, tt.quits, cmd != nil)
		})
	}
}

func Test_mcpConsoleModel_View_BeforeBanner(t *testing.T) {
	model := mcpConsoleModel{theme: terminal.NewTheme(terminal.AppearanceDark)}

	assert.Empty(t, model.View().Content)
}

func Test_mcpConsoleModel_state(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	tests := []struct {
		name     string
		state    mcp.InstanceState
		expected string
	}{
		{
			name:     "running",
			state:    mcp.InstanceStateRunning,
			expected: theme.StatusRunningStyle.Render("running"),
		},
		{
			name:     "not running",
			state:    mcp.InstanceStateNotRunning,
			expected: theme.StatusStoppedStyle.Render("not running"),
		},
		{
			name:     "unknown",
			state:    mcp.InstanceStateUnknown,
			expected: theme.StatusStartingStyle.Render("unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := mcpConsoleModel{theme: theme, banner: mcp.Banner{State: tt.state}}

			assert.Equal(t, tt.expected, model.state())
		})
	}
}

func Test_mcpConsoleModel_line(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	model := mcpConsoleModel{theme: theme}

	tests := []struct {
		name     string
		call     mcp.Call
		expected string
	}{
		{
			name: "a successful call",
			call: mcp.Call{
				Target:   "service api restart",
				Outcome:  "running",
				Duration: 2140 * time.Millisecond,
			},
			expected: " " + theme.StatusRunningStyle.Render("✔") + " service api restart  running   " + theme.PanelMutedStyle.Render("2.1s"),
		},
		{
			name: "a failed call",
			call: mcp.Call{
				Target:   "run",
				Failure:  "fuku run failed · exit 3",
				Duration: 7 * time.Millisecond,
			},
			expected: " " + theme.StatusFailedStyle.Render("✘") + " run                            " + theme.ErrorStyle.Render("fuku run failed · exit 3") + " · 7ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, model.line(tt.call))
		})
	}
}
