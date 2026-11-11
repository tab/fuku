package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"fuku/internal/adapters/cli"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// handleRun starts the profile through fuku run -d and returns its summary and states, or a note while it runs on
func (s *Server) handleRun(ctx context.Context, _ *sdk.CallToolRequest, in runInput) (*sdk.CallToolResult, runOutput, error) {
	dir, err := projectDir(in.Config)
	if err != nil {
		return nil, runOutput{}, err
	}

	args := append(s.withConfig([]string{cli.CommandRun.String(), "--" + cli.FlagDetached.String()}, in.Config, dir), "--", string(in.Profile))

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	stdout, err := s.execute.Execute(ctx, dir, args...)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, runOutput{ExitCode: exitKilled, Summary: noteStillStarting, InProgress: true}, nil
	}

	if err != nil {
		return nil, runOutput{}, err
	}

	return nil, runOutput{Summary: strings.TrimSpace(string(stdout)), Services: s.serviceStates(ctx, in.Config)}, nil
}

// handleStop stops the running profile through fuku stop, failing with the last lines of its output
func (s *Server) handleStop(ctx context.Context, _ *sdk.CallToolRequest, in stopInput) (*sdk.CallToolResult, commandOutput, error) {
	identity, err := identify(in.Config)
	if err != nil {
		return nil, commandOutput{}, err
	}

	status, err := s.status(identity, in.Config)
	if err != nil {
		return nil, commandOutput{}, err
	}

	args := append(s.withConfig([]string{cli.CommandStop.String()}, in.Config, identity.Project), "--", status.Profile)

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	stdout, err := s.execute.Execute(ctx, identity.Project, args...)
	if err != nil {
		return nil, commandOutput{}, &outputError{text: lastLines(strings.TrimSpace(string(stdout))+"\n"+err.Error(), failedLines), err: err}
	}

	return nil, commandOutput{Summary: "stopped profile " + status.Profile}, nil
}

// handleService applies the action to the named service through the API and returns the state it settled in
func (s *Server) handleService(ctx context.Context, _ *sdk.CallToolRequest, in serviceInput) (*sdk.CallToolResult, serviceOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	identity, err := identify(in.Config)
	if err != nil {
		return nil, serviceOutput{}, err
	}

	status, err := s.status(identity, in.Config)
	if err != nil {
		return nil, serviceOutput{}, err
	}

	project, err := s.project(in.Config, identity.Project)
	if err != nil {
		return nil, serviceOutput{}, err
	}

	client, err := s.find(ctx, status, project.Server, identity.Fingerprint)
	if errors.Is(err, contracts.ErrAPIUnreachable) {
		return nil, serviceOutput{}, errAPIOff
	}

	if err != nil {
		return nil, serviceOutput{}, err
	}

	services, err := client.Services(ctx)
	if err != nil {
		return nil, serviceOutput{}, explain(err)
	}

	id, err := serviceID(services, string(in.Name))
	if err != nil {
		return nil, serviceOutput{}, err
	}

	out, err := s.apply(ctx, client, identity, id, in.Action)
	if err != nil {
		return nil, serviceOutput{}, fmt.Errorf("failed to %s %s: %w", in.Action, in.Name, explain(err))
	}

	return nil, out, nil
}

// serviceStates reads the state of each service of the running profile once, or none when the state cannot be read
func (s *Server) serviceStates(ctx context.Context, config string) []serviceState {
	identity, err := identify(config)
	if err != nil {
		return nil
	}

	status, err := s.status(identity, config)
	if err != nil {
		return nil
	}

	project, err := s.project(config, identity.Project)
	if err != nil {
		return nil
	}

	out, err := s.state(ctx, status, project.Server, identity.Fingerprint)
	if err != nil {
		return nil
	}

	return out.Services
}

// apply runs the action and reports the settled state with the recent lines of a failed service, or the current state
func (s *Server) apply(ctx context.Context, client Client, identity model.Instance, id string, act contracts.Action) (serviceOutput, error) {
	settled, done, err := client.Act(ctx, id, act, serviceWait)
	if err != nil {
		return serviceOutput{}, err
	}

	if !done {
		current, err := client.Service(ctx, id)
		if err != nil {
			return serviceOutput{}, err
		}

		return serviceOutput{Name: current.Name, State: string(current.Status), PID: current.Process.PID, Watching: current.Watching, InProgress: true}, nil
	}

	out := serviceOutput{Name: settled.Name, State: string(settled.Status), PID: settled.Process.PID, Watching: settled.Watching}
	if settled.Status != model.StatusFailed {
		return out, nil
	}

	recent, err := s.read(ctx, identity, []string{settled.Name}, failedLines)
	if err != nil {
		return serviceOutput{}, fmt.Errorf("the service failed, and its logs cannot be read: %w", err)
	}

	for _, line := range recent.Lines {
		out.Lines = append(out.Lines, line.Message)
	}

	return out, nil
}

// outputError is a failed command whose text is the tail of its output, wrapping the failure with its exit code
type outputError struct {
	text string
	err  error
}

// Error returns the tail of the command output
func (e *outputError) Error() string {
	return e.text
}

// Unwrap returns the failure of the command
func (e *outputError) Unwrap() error {
	return e.err
}

// lastLines returns at most the newest n lines of a command output
func lastLines(output string, n int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

// serviceID maps the service name to its ID, failing with the names of the running profile when it has no such service
func serviceID(services []model.Service, name string) (string, error) {
	names := make([]string, 0, len(services))

	for _, service := range services {
		if service.Name == name {
			return service.ID, nil
		}

		names = append(names, service.Name)
	}

	return "", known([]string{name}, names)
}
