package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// methodCallTool is the JSON-RPC method of a tool call
const methodCallTool = "tools/call"

// Widths of the banner labels and of the call line columns: the tool with its target, then the outcome
const (
	bannerLabelWidth = 9
	callLabelWidth   = 20
	callOutcomeWidth = 9
)

// invalidArguments prefixes every schema failure of the SDK, whose text quotes the rejected argument value
const invalidArguments = `validating "arguments"`

// commandTools are the tools that run the fuku binary; a failure shows a fixed label, never the command's output
var commandTools = []string{toolRun, toolStop, toolDoctor}

// toolNames lists the tools in the order tools/list returns them
var toolNames = []string{toolDoctor, toolLogs, toolRun, toolService, toolStatus, toolStop}

// console writes the banner and one line per tool call to stderr, one write at a time
type console struct {
	theme  terminal.Theme
	stderr io.Writer
	mu     sync.Mutex
}

// write prints the lines in one write, with the styles stripped when stderr is no terminal
func (c *console) write(lines ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	lipgloss.Fprint(c.stderr, strings.Join(lines, "\n")+"\n")
}

// banner writes the startup banner with the state of the running instance the log socket reports
func (s *Server) banner() {
	identity, err := identify("")

	var status contracts.LogStatus
	if err == nil {
		status, err = s.dial(identity).Status()
	}

	s.console.write(bannerLines(s.console.theme, identity, status, err)...)
}

// logCalls is the receiving middleware that writes one stderr line per tool call
func (s *Server) logCalls(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		params, ok := req.GetParams().(*sdk.CallToolParamsRaw)
		if method != methodCallTool || !ok {
			return next(ctx, method, req)
		}

		started := time.Now()
		result, err := next(ctx, method, req)

		s.console.write(callLine(s.console.theme, params, result, err, time.Since(started)))

		return result, err
	}
}

// bannerLines renders the banner: the protocol, the project, the instance state, the tools and how to stop
func bannerLines(theme terminal.Theme, identity model.Instance, status contracts.LogStatus, err error) []string {
	label := func(name string) string {
		return theme.PanelMutedStyle.Render(terminal.PadRight(name, bannerLabelWidth))
	}

	return []string{
		fmt.Sprintf("%s fuku mcp · stdio · protocol %s", terminal.BoldStyle.Render("[+]"), sdk.SupportedProtocolVersions()[0]),
		fmt.Sprintf(" %s %s · %s", label("project"), identity.Project, identity.Fingerprint),
		fmt.Sprintf(" %s %s", label("instance"), instanceState(theme, status, err)),
		fmt.Sprintf(" %s %s", label("tools"), strings.Join(toolNames, " ")),
		"Serving MCP · close stdin or Ctrl+C to stop",
	}
}

// instanceState renders the running instance, its absence, or a short reason its state is unknown
func instanceState(theme terminal.Theme, status contracts.LogStatus, err error) string {
	switch {
	case errors.Is(err, contracts.ErrNoInstanceRunning):
		return theme.StatusStoppedStyle.Render("not running") + " · call run to start"
	case err != nil:
		return theme.StatusStartingStyle.Render(stateUnknown) + " · " + oneLine(err.Error())
	}

	state := fmt.Sprintf("%s · profile %s · pid %d", theme.StatusRunningStyle.Render("running"), status.Profile, status.PID)
	if status.API == "" {
		return state
	}

	return state + " · API " + status.API
}

// callLine renders one tool call: the tool, the service and action of a service call, the outcome and the duration
func callLine(theme terminal.Theme, params *sdk.CallToolParamsRaw, result sdk.Result, err error, duration time.Duration) string {
	called := toolResult(result)
	target, state := params.Name, ""

	if params.Name == toolService {
		target, state = serviceCall(params.Arguments, called)
	}

	target = terminal.PadRight(oneLine(target), callLabelWidth) + " "

	reason := failure(params.Name, called, err)
	if reason != "" {
		blank := strings.Repeat(" ", callOutcomeWidth+1)

		return fmt.Sprintf(" %s %s%s%s · %s", theme.StatusFailedStyle.Render("✘"), target, blank, theme.ErrorStyle.Render(reason), elapsed(duration))
	}

	outcome := terminal.PadRight(oneLine(state), callOutcomeWidth) + " "

	return fmt.Sprintf(" %s %s%s%s", theme.StatusRunningStyle.Render("✔"), target, outcome, theme.PanelMutedStyle.Render(elapsed(duration)))
}

// toolResult returns the tool result of a call, or an empty one when the call failed before it had one
func toolResult(result sdk.Result) *sdk.CallToolResult {
	called, ok := result.(*sdk.CallToolResult)
	if !ok || called == nil {
		return &sdk.CallToolResult{}
	}

	return called
}

// serviceCall returns the service name and action of a service call, and its settled or current state
func serviceCall(arguments json.RawMessage, called *sdk.CallToolResult) (string, string) {
	var in struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	}

	var out struct {
		State      string `json:"state"`
		InProgress bool   `json:"inProgress"`
	}

	_ = json.Unmarshal(arguments, &in)

	if structured, ok := called.StructuredContent.(json.RawMessage); ok {
		_ = json.Unmarshal(structured, &out)
	}

	target := strings.Join([]string{toolService, in.Name, in.Action}, " ")
	if !out.InProgress {
		return target, out.State
	}

	return target, out.State + " · in progress"
}

// failure returns why a call failed: a fixed label for bad arguments or a command tool, else the first error line
func failure(tool string, called *sdk.CallToolResult, err error) string {
	if err != nil {
		return oneLine(err.Error())
	}

	if !called.IsError {
		return ""
	}

	if called.GetError() != nil && strings.HasPrefix(called.GetError().Error(), invalidArguments) {
		return "invalid arguments"
	}

	if slices.Contains(commandTools, tool) {
		return commandFailure(tool, called.GetError())
	}

	for _, content := range called.Content {
		if text, ok := content.(*sdk.TextContent); ok {
			return oneLine(text.Text)
		}
	}

	return "the tool failed"
}

// commandFailure labels a failed command tool with its exit code when the command exited, and nothing of its output
func commandFailure(tool string, err error) string {
	label := "fuku " + tool + " failed"

	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return label
	}

	return fmt.Sprintf("%s · exit %d", label, exit.ExitCode())
}

// oneLine returns the first line of text without control characters, so a stderr line stays one plain line
func oneLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")

	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, line)
}

// elapsed formats a call duration in milliseconds below a second, and in seconds with one decimal above
func elapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	return fmt.Sprintf("%.1fs", d.Seconds())
}
