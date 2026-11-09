package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/adapters/cli"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// tool name values
const (
	toolStatus  = "status"
	toolLogs    = "logs"
	toolDoctor  = "doctor"
	toolRun     = "run"
	toolStop    = "stop"
	toolService = "service"
)

// action is what the service tool does to one service
type action string

// action values
const (
	actionStart   action = "start"
	actionStop    action = "stop"
	actionRestart action = "restart"
)

// tail is the number of log lines the logs tool returns
type tail int

// nonEmpty is a name the call must give, such as a profile or a service
type nonEmpty string

// Bounds of the logs tool: the lines it returns when the call names none, and the most it returns
const (
	tailDefault = 100
	tailMax     = 1000
)

// logsTimeout bounds the bounded log read of the logs tool; a read it cuts short is marked truncated
const logsTimeout = 10 * time.Second

// doctorTimeout bounds the fuku doctor run of the doctor tool
const doctorTimeout = 30 * time.Second

// commandTimeout bounds the fuku run and stop the tools execute, inside the 60s tool timeout of Codex
const commandTimeout = 50 * time.Second

// serviceWait is how long the service tool lets the API wait for the service to settle
const serviceWait = 45 * time.Second

// serviceTimeout bounds a whole service call, inside the 60s tool timeout of Codex
const serviceTimeout = 55 * time.Second

// failedLines is the number of recent log lines the service tool returns for a failed service
const failedLines = 20

// exitKilled is the exit code of a fuku run -d parent the run tool killed at its timeout
const exitKilled = -1

// noteStillStarting tells the agent the detached start outlived the run tool
const noteStillStarting = "fuku is still starting in the background; call status to follow it"

// stateUnknown is the state of a service, the phase of a profile and the mode of a run the instance cannot report
const stateUnknown = "unknown"

// noteWatching tells the agent in the status and service descriptions when a code edit needs no restart
const noteWatching = "a watching service restarts itself after a file change; after a code edit call status until the watching service whose files you edited shows a new PID or failed, then test it; restart only a service that is not watching"

// noteMode tells the agent in the status description what the mode of the running fuku allows
const noteMode = "mode detached is a background fuku, so stop then run applies a config change; mode tui, foreground or unknown is the developer's own terminal session: do not stop it to apply a config change, tell the developer the change needs their restart"

// noteInProgress tells the agent what an action that outlived its call means
const noteInProgress = "`inProgress: true` means the action outlived this call; follow it with status"

// noteNoAPI tells the agent why the status reports no service state
const noteNoAPI = "service states need the fuku REST API, which is off or unreachable"

// schemas give the named input types the enum and bounds inference cannot express
var schemas = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[action](): {
		Type: "string",
		Enum: []any{string(actionStart), string(actionStop), string(actionRestart)},
	},
	reflect.TypeFor[nonEmpty](): {
		Type:      "string",
		MinLength: new(1),
	},
	reflect.TypeFor[tail](): {
		Type:    "integer",
		Minimum: new(1.0),
		Maximum: new(float64(tailMax)),
	},
}

// target names the config file of one call, with the meaning of fuku --config
type target struct {
	Config string `json:"config,omitempty" jsonschema:"Omit unless the user names a config file other than fuku.yaml"`
}

// statusInput is the input of the status tool
type statusInput struct {
	target
}

// logsInput is the input of the logs tool
type logsInput struct {
	target
	Services []string `json:"services,omitempty" jsonschema:"the service names to read; every service by default"`
	Tail     tail     `json:"tail,omitempty"     jsonschema:"the newest lines to return, 100 by default and 1000 at most"`
}

// doctorInput is the input of the doctor tool
type doctorInput struct {
	target
	Profile string `json:"profile,omitempty" jsonschema:"the profile to diagnose; the default profile by default"`
}

// runInput is the input of the run tool
type runInput struct {
	target
	Profile nonEmpty `json:"profile" jsonschema:"the profile to run; default unless the user names another"`
}

// stopInput is the input of the stop tool
type stopInput struct {
	target
}

// serviceInput is the input of the service tool
type serviceInput struct {
	target
	Name   nonEmpty `json:"name"   jsonschema:"the service name from fuku.yaml"`
	Action action   `json:"action" jsonschema:"what to do to the service"`
}

// statusOutput is the state of the running profile
type statusOutput struct {
	Profile  string         `json:"profile"`
	Phase    string         `json:"phase"`
	Mode     string         `json:"mode"`
	Services []serviceState `json:"services"`
	Note     string         `json:"note,omitempty"`
}

// serviceState is the state of one service of the running profile
type serviceState struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	PID      int    `json:"pid"`
	Uptime   int64  `json:"uptime"`
	Watching bool   `json:"watching"`
}

// logsOutput is a bounded read of the log buffer
type logsOutput struct {
	Lines     []logLine `json:"lines"`
	Truncated bool      `json:"truncated,omitempty"`
}

// logLine is one line of service output
type logLine struct {
	Service string `json:"service"`
	Message string `json:"message"`
}

// doctorOutput is the JSON report of fuku doctor --json
type doctorOutput map[string]any

// commandOutput is the outcome of a fuku command the tool executed
type commandOutput struct {
	ExitCode   int    `json:"exitCode"`
	Summary    string `json:"summary"`
	InProgress bool   `json:"inProgress,omitempty"`
}

// runOutput is the outcome of fuku run with the state of each service once the start ended
type runOutput struct {
	ExitCode   int            `json:"exitCode"`
	Summary    string         `json:"summary"`
	InProgress bool           `json:"inProgress,omitempty"`
	Services   []serviceState `json:"services,omitempty"`
}

// serviceOutput is the state a service settled in after the action, or its current state while the action runs
type serviceOutput struct {
	Name       string   `json:"name"`
	State      string   `json:"state"`
	PID        int      `json:"pid"`
	Watching   bool     `json:"watching"`
	InProgress bool     `json:"inProgress,omitempty"`
	Lines      []string `json:"lines,omitempty"`
}

// register adds the six tools, which the SDK lists in its fixed name order
func (s *Server) register() error {
	readOnly := &sdk.ToolAnnotations{ReadOnlyHint: true}
	destructive := &sdk.ToolAnnotations{DestructiveHint: new(true)}
	additive := &sdk.ToolAnnotations{DestructiveHint: new(false)}

	return errors.Join(
		add(s.server, toolStatus, "Show the running profile, its phase, its mode and each service's state, PID, uptime and watching; "+noteMode+"; "+noteWatching, readOnly, s.handleStatus),
		add(s.server, toolLogs, "Read the newest log lines of the running services; never follows", readOnly, s.handleLogs),
		add(s.server, toolDoctor, "Diagnose the config, the environment and the running instance; use after a failed run or a config edit; not needed before a plain run", readOnly, s.handleDoctor),
		add(s.server, toolRun, "Start a profile in the background and return once every service runs; returns every service's state; call status only when inProgress is true or services is missing; inProgress means the start outlived this call", additive, s.handleRun),
		add(s.server, toolStop, "Stop the running fuku and every service of its profile; leave fuku running after the task; stop only when the user asks, or to apply a config change when status reports mode detached", destructive, s.handleStop),
		add(s.server, toolService, "Start, stop or restart one service and return once it settles; returns the settled state; no status call needed after; "+noteWatching+"; "+noteInProgress, additive, s.handleService),
	)
}

// add registers one tool with the input schema of its input type
func add[In, Out any](server *sdk.Server, name, description string, annotations *sdk.ToolAnnotations, handler sdk.ToolHandlerFor[In, Out]) error {
	schema, err := jsonschema.For[In](&jsonschema.ForOptions{TypeSchemas: schemas})
	if err != nil {
		return fmt.Errorf("failed to infer the input schema of %s: %w", name, err)
	}

	sdk.AddTool(server, &sdk.Tool{Name: name, Description: description, Annotations: annotations, InputSchema: schema}, handler)

	return nil
}

// handleStatus reports the profile, its phase and each service's state, or the names alone without an API or a config
func (s *Server) handleStatus(ctx context.Context, _ *sdk.CallToolRequest, in statusInput) (*sdk.CallToolResult, statusOutput, error) {
	identity, err := identify(in.Config)
	if err != nil {
		return nil, statusOutput{}, err
	}

	status, err := s.status(identity, in.Config)
	if err != nil {
		return nil, statusOutput{}, err
	}

	project, err := s.project(in.Config, identity.Project)
	if err != nil {
		out := unknownState(status)
		out.Note = oneLine(err.Error())

		return nil, out, nil
	}

	out, err := s.state(ctx, status, project.Server, identity.Fingerprint)
	if errors.Is(err, contracts.ErrAPIUnreachable) {
		return nil, unknownState(status), nil
	}

	if err != nil {
		return nil, statusOutput{}, err
	}

	return nil, out, nil
}

// handleLogs returns at most tail of the newest lines of the named services, whatever the config, and never follows
func (s *Server) handleLogs(ctx context.Context, _ *sdk.CallToolRequest, in logsInput) (*sdk.CallToolResult, logsOutput, error) {
	identity, err := identify(in.Config)
	if err != nil {
		return nil, logsOutput{}, err
	}

	status, err := s.status(identity, in.Config)
	if err != nil {
		return nil, logsOutput{}, err
	}

	if err := known(in.Services, status.Services); err != nil {
		return nil, logsOutput{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, logsTimeout)
	defer cancel()

	out, err := s.read(ctx, identity, in.Services, cmp.Or(int(in.Tail), tailDefault))
	if err != nil {
		return nil, logsOutput{}, fmt.Errorf("failed to read the logs of fuku in %s: %w", identity.Project, err)
	}

	return nil, out, nil
}

// handleDoctor runs fuku doctor --json in the project, whatever its config, and returns the report
func (s *Server) handleDoctor(ctx context.Context, _ *sdk.CallToolRequest, in doctorInput) (*sdk.CallToolResult, doctorOutput, error) {
	dir, err := projectDir(in.Config)
	if err != nil {
		return nil, nil, err
	}

	args := []string{cli.CommandDoctor.String()}
	if in.Profile != "" {
		args = append(args, in.Profile)
	}

	args = withConfig(append(args, "--"+cli.FlagJSON.String()), in.Config, dir)

	ctx, cancel := context.WithTimeout(ctx, doctorTimeout)
	defer cancel()

	stdout, runErr := s.execute.Execute(ctx, dir, args...)

	var report doctorOutput
	if err := json.Unmarshal(stdout, &report); err != nil {
		return nil, nil, fmt.Errorf("failed to read the doctor report: %w", errors.Join(runErr, err))
	}

	return nil, report, nil
}

// status reads the status of the instance serving the project of the call, naming the profiles to run when none runs
func (s *Server) status(identity model.Instance, config string) (contracts.LogStatus, error) {
	status, err := s.dial(identity).Status()
	if errors.Is(err, contracts.ErrNoInstanceRunning) {
		return contracts.LogStatus{}, fmt.Errorf("%w in %s; %s", contracts.ErrNoInstanceRunning, identity.Project, s.runHint(config, identity.Project))
	}

	if err != nil {
		return contracts.LogStatus{}, fmt.Errorf("failed to read the status of fuku in %s: %w", identity.Project, err)
	}

	return status, nil
}

// runHint tells the agent to call run, with the profile names of the config when it loads
func (s *Server) runHint(config, dir string) string {
	loaded := s.load(configFile(config, dir))
	if loaded.Error != nil || len(loaded.Project.Profiles) == 0 {
		return "call run first"
	}

	return "call run with one of: " + strings.Join(slices.Sorted(maps.Keys(loaded.Project.Profiles)), ", ")
}

// project loads the config of the call fresh from its project directory, failing on an invalid config
func (s *Server) project(config, dir string) (model.Project, error) {
	loaded := s.load(configFile(config, dir))
	if loaded.Error != nil {
		return model.Project{}, fmt.Errorf("failed to load the config in %s: %w", dir, loaded.Error)
	}

	return loaded.Project, nil
}

// state reads the profile, its phase and the state of each service from the API of the instance
func (s *Server) state(ctx context.Context, status contracts.LogStatus, server model.Server, fingerprint string) (statusOutput, error) {
	api, err := s.findAPI(ctx, status, server, fingerprint)
	if err != nil {
		return statusOutput{}, err
	}

	snapshot, err := api.Status(ctx)
	if err != nil {
		return statusOutput{}, explain(err)
	}

	list, err := api.Services(ctx)
	if err != nil {
		return statusOutput{}, explain(err)
	}

	services := make([]serviceState, 0, len(list))
	for _, service := range list {
		services = append(services, serviceState{
			Name:     service.Name,
			State:    string(service.Status),
			PID:      service.Process.PID,
			Uptime:   uptime(service.Process.StartedAt),
			Watching: service.Watching,
		})
	}

	return statusOutput{Profile: snapshot.Profile, Phase: string(snapshot.Phase), Mode: mode(status), Services: services}, nil
}

// unknownState reports the profile and service names of the status frame, with no state
func unknownState(status contracts.LogStatus) statusOutput {
	services := make([]serviceState, 0, len(status.Services))
	for _, name := range status.Services {
		services = append(services, serviceState{Name: name, State: stateUnknown})
	}

	return statusOutput{Profile: status.Profile, Phase: stateUnknown, Mode: mode(status), Services: services, Note: noteNoAPI}
}

// mode reports the run mode of the status frame, unknown for an older fuku that sends none
func mode(status contracts.LogStatus) string {
	return cmp.Or(string(status.Mode), stateUnknown)
}

// known fails on a requested service name the running profile does not have, listing the names it has
func known(requested, names []string) error {
	var unknown []string

	for _, name := range requested {
		if !slices.Contains(names, name) {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) == 0 {
		return nil
	}

	return fmt.Errorf("%w %s; the running profile has %s", errUnknownService, strings.Join(unknown, ", "), strings.Join(names, ", "))
}

// read collects the newest tail lines with no follow until the instance ends the stream, marking a read ctx cut short
func (s *Server) read(ctx context.Context, identity model.Instance, services []string, tail int) (logsOutput, error) {
	client := s.dial(identity)

	if err := client.Connect(); err != nil {
		return logsOutput{}, err
	}

	defer client.Close()

	if err := client.Subscribe(services, model.ReplayOptions{Tail: &tail, NoFollow: true}); err != nil {
		return logsOutput{}, err
	}

	collected := &collector{lines: make([]logLine, 0, tail)}
	if err := client.Stream(ctx, collected); err != nil {
		return logsOutput{}, err
	}

	return logsOutput{Lines: collected.lines, Truncated: ctx.Err() != nil}, nil
}

// collector keeps the lines of a bounded log read
type collector struct {
	lines []logLine
}

// HandleStatus accepts the acknowledgement of the read
func (c *collector) HandleStatus(contracts.LogStatus) error {
	return nil
}

// HandleLog keeps one line
func (c *collector) HandleLog(line model.LogLine) {
	c.lines = append(c.lines, logLine{Service: line.Service, Message: line.Message})
}
