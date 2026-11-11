package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
	"unicode"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// methodCallTool is the JSON-RPC method of a tool call
const methodCallTool = "tools/call"

// invalidArguments prefixes every schema failure of the SDK, whose text quotes the rejected argument value
const invalidArguments = `validating "arguments"`

// commandTools are the tools that run the fuku binary; a failure shows a fixed label, never the command's output
var commandTools = []string{toolRun, toolStop, toolDoctor}

// toolNames lists the tools in the order tools/list returns them
var toolNames = []string{toolConfig, toolDoctor, toolLogs, toolRun, toolService, toolStatus, toolStop}

// InstanceState is the state of the project's instance the banner shows
type InstanceState string

// InstanceState values
const (
	InstanceStateRunning    InstanceState = "running"
	InstanceStateNotRunning InstanceState = "not running"
	InstanceStateUnknown    InstanceState = stateUnknown
)

// Banner is the startup record: the protocol, the project, the state of its instance and the tools
type Banner struct {
	Protocol    string
	Project     string
	Fingerprint string
	State       InstanceState
	Detail      string
	Tools       []string
}

// Call is the record of one tool call: its target, its settled state or why it failed, and its duration
type Call struct {
	Target   string
	Outcome  string
	Failure  string
	Duration time.Duration
}

// Console shows the banner and one line per tool call on stderr, between Open and Close
type Console interface {
	Open()
	Banner(banner Banner)
	Call(call Call)
	Close()
}

// Elapsed formats the duration in milliseconds below a second, and in seconds with one decimal above
func (c Call) Elapsed() string {
	if c.Duration < time.Second {
		return fmt.Sprintf("%dms", c.Duration.Milliseconds())
	}

	return fmt.Sprintf("%.1fs", c.Duration.Seconds())
}

// banner shows the startup banner with the state of the running instance the log socket reports
func (s *Server) banner() {
	identity, err := identify("")

	var status contracts.LogStatus
	if err == nil {
		status, err = s.instance(identity).Status()
	}

	s.console.Banner(bannerRecord(identity, status, err))
}

// logCalls is the receiving middleware that shows one line per tool call, with the API tokens redacted
func (s *Server) logCalls(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		params, ok := req.GetParams().(*sdk.CallToolParamsRaw)
		if method != methodCallTool || !ok {
			return next(ctx, method, req)
		}

		started := time.Now()
		result, err := next(ctx, method, req)

		call := callRecord(params, result, err, time.Since(started))
		tokens := s.tokens(params.Arguments)
		call.Target, call.Outcome, call.Failure = redact(call.Target, tokens), redact(call.Outcome, tokens), redact(call.Failure, tokens)

		s.console.Call(call)

		return result, err
	}
}

// bannerRecord returns the banner: the protocol, the project, the instance state and the tools
func bannerRecord(identity model.Instance, status contracts.LogStatus, err error) Banner {
	banner := Banner{
		Protocol:    sdk.SupportedProtocolVersions()[0],
		Project:     identity.Project,
		Fingerprint: identity.Fingerprint,
		Tools:       toolNames,
	}

	switch {
	case errors.Is(err, contracts.ErrNoInstanceRunning):
		banner.State, banner.Detail = InstanceStateNotRunning, "call run to start"
	case err != nil:
		banner.State, banner.Detail = InstanceStateUnknown, oneLine(err.Error())
	case status.API == "":
		banner.State, banner.Detail = InstanceStateRunning, fmt.Sprintf("profile %s · pid %d", status.Profile, status.PID)
	default:
		banner.State, banner.Detail = InstanceStateRunning, fmt.Sprintf("profile %s · http://%s · pid %d", status.Profile, status.API, status.PID)
	}

	return banner
}

// callRecord returns one tool call: the tool, the service and action of a service call, the outcome and the duration
func callRecord(params *sdk.CallToolParamsRaw, result sdk.Result, err error, duration time.Duration) Call {
	called := toolResult(result)
	target, state := params.Name, ""

	if params.Name == toolService {
		target, state = serviceCall(params.Arguments, called)
	}

	reason := failure(params.Name, called, err)
	if reason != "" {
		return Call{Target: oneLine(target), Failure: reason, Duration: duration}
	}

	return Call{Target: oneLine(target), Outcome: oneLine(state), Duration: duration}
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
