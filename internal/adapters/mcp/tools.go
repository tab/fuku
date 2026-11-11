package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/adapters/cli"
	"fuku/internal/app/profiles"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Tool name constants
const (
	toolStatus  = "status"
	toolLogs    = "logs"
	toolDoctor  = "doctor"
	toolConfig  = "config"
	toolRun     = "run"
	toolStop    = "stop"
	toolService = "service"
)

// tail is the number of log lines the logs tool returns
type tail int

// nonEmpty is a name the call must give, such as a profile or a service
type nonEmpty string

// Logs tool line bound constants
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
const noteWatching = "a watching service restarts itself after a file change; after a code edit restart the edited service when it is not watching; when it is watching, call status until it shows a new PID or failed, then test it, without restarting it yourself; restart any service the user asks you to"

// noteOnlyTool tells the agent in each control tool's description that the tool is the only way to do its job
const noteOnlyTool = "use this tool, never the fuku CLI or a script"

// noteMode tells the agent in the status description what the mode of the running fuku allows
const noteMode = "mode detached is a background fuku, so stop then run applies a config change; mode tui, foreground or unknown is the developer's own terminal session: do not stop it to apply a config change, tell the developer the change needs their restart"

// noteInProgress tells the agent what an action that outlived its call means
const noteInProgress = "`inProgress: true` means the action outlived this call; follow it with status"

// noteNoAPI tells the agent why the status reports no service state
const noteNoAPI = "service states need the fuku REST API, which is off or unreachable"

// schemas give the named input types the enum and bounds inference cannot express
var schemas = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[contracts.Action](): {
		Type: "string",
		Enum: []any{string(contracts.ActionStart), string(contracts.ActionStop), string(contracts.ActionRestart)},
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

// configInput is the input of the config tool
type configInput struct {
	target
	Service string `json:"service,omitempty" jsonschema:"one service name; every service by default"`
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
	Name   nonEmpty         `json:"name"   jsonschema:"the service name from fuku.yaml"`
	Action contracts.Action `json:"action" jsonschema:"what to do to the service"`
}

// statusOutput is the state of the running profile, or the profiles to run when none runs
type statusOutput struct {
	Running  bool           `json:"running"`
	Profile  string         `json:"profile"`
	Phase    string         `json:"phase"`
	Mode     string         `json:"mode"`
	Profiles []string       `json:"profiles,omitempty"`
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

// configOutput is the effective config: the services of each profile, each service's settings and the API state
type configOutput struct {
	Profiles map[string][]string `json:"profiles"`
	Services []serviceConfig     `json:"services"`
	API      apiConfig           `json:"api"`
}

// serviceConfig is the effective config of one service
type serviceConfig struct {
	Name      string           `json:"name"`
	Command   string           `json:"command"`
	Dir       string           `json:"dir"`
	Tier      string           `json:"tier"`
	Readiness *readinessConfig `json:"readiness,omitempty"`
	Watch     *watchConfig     `json:"watch,omitempty"`
	EnvFiles  []string         `json:"envFiles,omitempty"`
}

// readinessConfig is the readiness probe of one service: its type, the address, URL or pattern it checks, its timeout
type readinessConfig struct {
	Type    string `json:"type"`
	Target  string `json:"target"`
	Timeout string `json:"timeout"`
}

// watchConfig is the file watch of one service
type watchConfig struct {
	Include  []string `json:"include,omitempty"`
	Ignore   []string `json:"ignore,omitempty"`
	Shared   []string `json:"shared,omitempty"`
	Debounce string   `json:"debounce"`
}

// apiConfig says whether the REST API is on and where it listens, never its token
type apiConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
}

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

// register adds the seven tools, which the SDK lists in its fixed name order
func (s *Server) register() error {
	readOnly := &sdk.ToolAnnotations{ReadOnlyHint: true}
	destructive := &sdk.ToolAnnotations{DestructiveHint: new(true)}

	return errors.Join(
		add(s.server, toolStatus, "Show the running profile, its phase, its mode and each service's state, PID, uptime and watching; "+noteOnlyTool+"; works when nothing runs: then returns running false with the profiles to run; "+noteMode+"; "+noteWatching, readOnly, s.handleStatus),
		add(s.server, toolLogs, "Read the newest log lines of the running services; "+noteOnlyTool+"; never follows, and no tool follows logs live: say so instead of a workaround", readOnly, s.handleLogs),
		add(s.server, toolDoctor, "Diagnose the config, the environment and the running instance; use after a failed run or a config edit; not needed before a plain run", readOnly, s.handleDoctor),
		add(s.server, toolConfig, "Show the effective config, fuku.yaml merged with fuku.override.yaml: each profile's services and each service's command, dir, tier, readiness, watch and env files, and whether the API is on; "+noteOnlyTool+"; read it instead of fuku.yaml; it never shows the API token", readOnly, s.handleConfig),
		add(s.server, toolRun, "Start a profile in the background and return once every service runs; "+noteOnlyTool+"; returns every service's state; call status only when inProgress is true or services is missing; inProgress means the start outlived this call", destructive, s.handleRun),
		add(s.server, toolStop, "Stop the running fuku and every service of its profile; "+noteOnlyTool+"; leave fuku running after the task; stop only when the user asks, or to apply a config change when status reports mode detached", destructive, s.handleStop),
		add(s.server, toolService, "Start, stop or restart one service and return once it settles; "+noteOnlyTool+"; returns the settled state; no status call needed after; "+noteWatching+"; "+noteInProgress, destructive, s.handleService),
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
	if errors.Is(err, contracts.ErrNoInstanceRunning) {
		return nil, s.idle(in.Config, identity.Project), nil
	}

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

	args := s.withConfig([]string{cli.CommandDoctor.String(), "--" + cli.FlagJSON.String()}, in.Config, dir)
	if in.Profile != "" {
		args = append(args, "--", in.Profile)
	}

	ctx, cancel := context.WithTimeout(ctx, doctorTimeout)
	defer cancel()

	stdout, runErr := s.execute.Execute(ctx, dir, args...)

	var report doctorOutput
	if err := json.Unmarshal(stdout, &report); err != nil {
		return nil, nil, fmt.Errorf("failed to read the doctor report: %w", errors.Join(runErr, err))
	}

	return nil, report, nil
}

// handleConfig loads the config fresh and returns its profiles, its services and whether the API is on
func (s *Server) handleConfig(_ context.Context, _ *sdk.CallToolRequest, in configInput) (*sdk.CallToolResult, configOutput, error) {
	dir, err := projectDir(in.Config)
	if err != nil {
		return nil, configOutput{}, err
	}

	project, err := s.project(in.Config, dir)
	if err != nil {
		return nil, configOutput{}, err
	}

	services, err := selected(project.Services, in.Service)
	if err != nil {
		return nil, configOutput{}, err
	}

	profileServices, err := resolveProfiles(project)
	if err != nil {
		return nil, configOutput{}, err
	}

	out := configOutput{
		Profiles: profileServices,
		Services: make([]serviceConfig, 0, len(services)),
		API:      apiConfig{Enabled: project.Server.Listen != "", Listen: project.Server.Listen},
	}

	for _, service := range services {
		out.Services = append(out.Services, effective(service, dir))
	}

	return nil, out, nil
}

// status reads the status of the instance serving the project of the call, naming the profiles to run when none runs
func (s *Server) status(identity model.Instance, config string) (contracts.LogStatus, error) {
	status, err := s.instance(identity).Status()
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
	return hint(s.profiles(config, dir))
}

// profiles returns the sorted profile names of the config, or none when it does not load
func (s *Server) profiles(config, dir string) []string {
	loaded := s.load(dir, configName(config))
	if loaded.Error != nil {
		return nil
	}

	return slices.Sorted(maps.Keys(loaded.Project.Profiles))
}

// hint tells the agent to call run with one of the profile names, or plainly without any
func hint(profiles []string) string {
	if len(profiles) == 0 {
		return "call run first"
	}

	return "call run with one of: " + strings.Join(profiles, ", ")
}

// idle reports that no fuku runs in the project, with the profiles of the config to run
func (s *Server) idle(config, dir string) statusOutput {
	names := s.profiles(config, dir)

	return statusOutput{Profiles: names, Services: []serviceState{}, Note: "no fuku runs in " + dir + "; " + hint(names)}
}

// project loads the config of the call fresh from its project directory, failing on an invalid config
func (s *Server) project(config, dir string) (model.Project, error) {
	loaded := s.load(dir, configName(config))
	if loaded.Error != nil {
		return model.Project{}, fmt.Errorf("failed to load the config in %s: %w", dir, loaded.Error)
	}

	return loaded.Project, nil
}

// state reads the profile, its phase and the state of each service from the API of the instance
func (s *Server) state(ctx context.Context, status contracts.LogStatus, server model.Server, fingerprint string) (statusOutput, error) {
	client, err := s.find(ctx, status, server, fingerprint)
	if err != nil {
		return statusOutput{}, err
	}

	snapshot, err := client.Status(ctx)
	if err != nil {
		return statusOutput{}, explain(err)
	}

	list, err := client.Services(ctx)
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

	return statusOutput{Running: true, Profile: snapshot.Profile, Phase: string(snapshot.Phase), Mode: mode(status), Services: services}, nil
}

// unknownState reports the profile and service names of the status frame, with no state
func unknownState(status contracts.LogStatus) statusOutput {
	services := make([]serviceState, 0, len(status.Services))
	for _, name := range status.Services {
		services = append(services, serviceState{Name: name, State: stateUnknown})
	}

	return statusOutput{Running: true, Profile: status.Profile, Phase: stateUnknown, Mode: mode(status), Services: services, Note: noteNoAPI}
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

// selected returns every service, or the one the call names, failing with the config's names for an unknown one
func selected(services []model.Service, name string) ([]model.Service, error) {
	if name == "" {
		return services, nil
	}

	names := make([]string, 0, len(services))

	for _, service := range services {
		if service.Name == name {
			return []model.Service{service}, nil
		}

		names = append(names, service.Name)
	}

	return nil, fmt.Errorf("%w %s; the config has %s", errUnknownService, name, strings.Join(names, ", "))
}

// resolveProfiles maps each profile to the names of the services it runs, in config order
func resolveProfiles(project model.Project) (map[string][]string, error) {
	resolver := profiles.NewResolver(project)
	out := make(map[string][]string, len(project.Profiles))

	for profile := range project.Profiles {
		tiers, err := resolver.Resolve(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve profile %s: %w", profile, err)
		}

		names := []string{}

		for _, tier := range tiers {
			for _, service := range tier.Services {
				names = append(names, service.Name)
			}
		}

		out[profile] = names
	}

	return out, nil
}

// effective presents the config of one service, with its dir relative to the project when it is inside it
func effective(service model.Service, dir string) serviceConfig {
	out := serviceConfig{Name: service.Name, Command: service.Command, Dir: service.Directory, Tier: service.Tier}

	if rel, err := filepath.Rel(dir, service.Directory); err == nil && filepath.IsAbs(service.Directory) && filepath.IsLocal(rel) {
		out.Dir = rel
	}

	if service.Readiness != nil {
		out.Readiness = &readinessConfig{
			Type:    string(service.Readiness.Type),
			Target:  probeTarget(*service.Readiness),
			Timeout: service.Readiness.Timeout.String(),
		}
	}

	if service.Watch != nil {
		out.Watch = &watchConfig{Include: service.Watch.Include, Ignore: service.Watch.Ignore, Shared: service.Watch.Shared, Debounce: service.Watch.Debounce.String()}
	}

	if service.Environment != nil {
		out.EnvFiles = service.Environment.Files
	}

	return out
}

// probeTarget returns what the readiness probe checks: the address, the URL or the log pattern
func probeTarget(readiness model.Readiness) string {
	switch readiness.Type {
	case model.ReadinessTCP:
		return readiness.Address
	case model.ReadinessHTTP:
		return readiness.URL
	case model.ReadinessLog:
		return readiness.Pattern
	}

	return ""
}

// read collects the newest tail lines with no follow until the instance ends the stream, marking a read ctx cut short
func (s *Server) read(ctx context.Context, identity model.Instance, services []string, tail int) (logsOutput, error) {
	instance := s.instance(identity)

	if err := instance.Connect(); err != nil {
		return logsOutput{}, err
	}

	defer instance.Close()

	if err := instance.Subscribe(services, model.ReplayOptions{Tail: &tail, NoFollow: true}); err != nil {
		return logsOutput{}, err
	}

	collected := &collector{lines: make([]logLine, 0, tail)}
	if err := instance.Stream(ctx, collected); err != nil {
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
