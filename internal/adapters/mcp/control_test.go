package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/adapters/terminal"
	"fuku/internal/app/logs"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Server_handleRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockExecutor := NewMockExecutor(ctrl)
	mockAPI := NewMockAPI(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	expired, cancel := context.WithDeadline(t.Context(), time.Time{})
	defer cancel()

	token := "secret-token"
	address := "127.0.0.1:3858"
	startedAt := time.Now().Add(-3*time.Second - 500*time.Millisecond)
	summary := "Running detached · pid 42 · 2 services · 3.1s\nAPI " + address
	frame := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}, API: address}
	services := []model.Service{
		{ID: "u-1", Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42, StartedAt: startedAt}},
		{ID: "u-2", Name: "db", Status: model.StatusRunning, Process: model.Process{PID: 43, StartedAt: startedAt}},
	}
	states := []serviceState{{Name: "api", State: "running", PID: 42, Uptime: 3}, {Name: "db", State: "running", PID: 43, Uptime: 3}}
	readsState := func() {
		gomock.InOrder(
			mockAPI.EXPECT().Status(gomock.Any()).Return(model.Snapshot{Profile: "default", Phase: model.PhaseRunning}, nil),
			mockAPI.EXPECT().Services(gomock.Any()).Return(services, nil),
		)
	}
	dial := func(model.Instance) Client { return mockClient }
	dialAPI := func(dialedAddress, dialedToken string) (API, error) {
		assert.Equal(t, address, dialedAddress)
		assert.Equal(t, token, dialedToken)

		return mockAPI, nil
	}
	load := func(string) model.Config {
		return model.Config{Project: model.Project{Server: model.Server{Token: token}}}
	}

	subject, err := NewServer(dial, dialAPI, load, mockExecutor, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	tests := []struct {
		name          string
		before        func() context.Context
		files         []string
		in            runInput
		expected      runOutput
		expectedError string
	}{
		{
			name: "the profile starts",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(" • api Starting\n"+summary+"\n"), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			in:       runInput{Profile: "default"},
			expected: runOutput{Summary: "• api Starting\n" + summary, Services: states},
		},
		{
			name: "the profile starts and its state cannot be read",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				mockAPI.EXPECT().Status(gomock.Any()).Return(model.Snapshot{}, fmt.Errorf("%w: connection refused", contracts.ErrAPIUnreachable))

				return t.Context()
			},
			in:       runInput{Profile: "default"},
			expected: runOutput{Summary: summary},
		},
		{
			name: "the profile starts and its status frame cannot be read",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, errors.New("timeout"))

				return t.Context()
			},
			in:       runInput{Profile: "default"},
			expected: runOutput{Summary: summary},
		},
		{
			name: "a config file the call names",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "backend", "--detached", "--config", filepath.Join(dir, "custom.yaml")).Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			in:       runInput{target: target{Config: "custom.yaml"}, Profile: "backend"},
			expected: runOutput{Summary: summary, Services: states},
		},
		{
			name: "the root fuku.yaml the call names",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			files:    []string{"fuku.yaml"},
			in:       runInput{target: target{Config: "fuku.yaml"}, Profile: "default"},
			expected: runOutput{Summary: summary, Services: states},
		},
		{
			name: "the root fuku.yml the call names without a fuku.yaml",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			files:    []string{"fuku.yml"},
			in:       runInput{target: target{Config: "./fuku.yml"}, Profile: "default"},
			expected: runOutput{Summary: summary, Services: states},
		},
		{
			name: "the root fuku.yml the call names beside a fuku.yaml",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached", "--config", filepath.Join(dir, "fuku.yml")).Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			files:    []string{"fuku.yaml", "fuku.yml"},
			in:       runInput{target: target{Config: "fuku.yml"}, Profile: "default"},
			expected: runOutput{Summary: summary, Services: states},
		},
		{
			name: "the root fuku.yaml the call names beside a fuku.yml",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return([]byte(summary), nil)
				mockClient.EXPECT().Status().Return(frame, nil)
				readsState()

				return t.Context()
			},
			files:    []string{"fuku.yaml", "fuku.yml"},
			in:       runInput{target: target{Config: "fuku.yaml"}, Profile: "default"},
			expected: runOutput{Summary: summary, Services: states},
		},
		{
			name: "fuku already runs",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return(nil, errors.New("fuku run default --detached failed: exit status 1: fuku is already running in "+dir))

				return t.Context()
			},
			in:            runInput{Profile: "default"},
			expectedError: "fuku run default --detached failed: exit status 1: fuku is already running in " + dir,
		},
		{
			name: "the start outlives the tool",
			before: func() context.Context {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "default", "--detached").Return(nil, errors.New("fuku run default --detached failed: signal: killed"))

				return expired
			},
			in:       runInput{Profile: "default"},
			expected: runOutput{ExitCode: exitKilled, Summary: noteStillStarting, InProgress: true},
		},
		{
			name:          "a missing config directory",
			before:        func() context.Context { return t.Context() },
			in:            runInput{target: target{Config: "missing/fuku.yaml"}, Profile: "default"},
			expectedError: instance.ErrFailedToResolveProject.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, file := range tt.files {
				path := filepath.Join(dir, file)
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				t.Cleanup(func() { require.NoError(t, os.Remove(path)) })
			}

			ctx := tt.before()

			_, result, err := subject.handleRun(ctx, nil, tt.in)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				assert.NotContains(t, err.Error(), token)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), token)
		})
	}
}

func Test_Server_handleStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	frame := contracts.LogStatus{Profile: "backend", Services: []string{"api", "db"}}
	unresolved := "failed to resolve project directory: lstat " + filepath.Join(dir, "missing") + ": no such file or directory"
	stopped := []byte("fuku | [FX] provided\nStopping fuku · pid 42 · profile backend ... stopped in 0.4s\nfuku | [FX] OnStop hook executed\n")

	var noisy, tail strings.Builder

	for i := range 25 {
		fmt.Fprintf(&noisy, "fuku | [FX] line %d\n", i)
	}

	noisy.WriteString("Stopping fuku · pid 42 · profile backend ... failed\n")

	for i := 7; i < 25; i++ {
		fmt.Fprintf(&tail, "fuku | [FX] line %d\n", i)
	}

	tail.WriteString("Stopping fuku · pid 42 · profile backend ... failed\nfuku stop backend failed: exit status 1: cannot reach the running fuku, nothing was stopped")

	dial := func(model.Instance) Client { return mockClient }

	var dialAPI APIDialer

	load := func(string) model.Config { return model.Config{} }
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(dial, dialAPI, load, mockExecutor, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	tests := []struct {
		name            string
		before          func()
		files           []string
		arguments       map[string]any
		expected        string
		expectedIsError bool
	}{
		{
			name: "the running profile stops",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(stopped, nil),
				)
			},
			expected: `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "a config file the call names",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend", "--config", filepath.Join(dir, "custom.yaml")).Return(stopped, nil),
				)
			},
			arguments: map[string]any{"config": "custom.yaml"},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "the root fuku.yaml the call names",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(stopped, nil),
				)
			},
			files:     []string{"fuku.yaml"},
			arguments: map[string]any{"config": "fuku.yaml"},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "an absolute path to the root fuku.yaml",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(stopped, nil),
				)
			},
			files:     []string{"fuku.yaml"},
			arguments: map[string]any{"config": filepath.Join(dir, "fuku.yaml")},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "the root fuku.yml the call names without a fuku.yaml",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(stopped, nil),
				)
			},
			files:     []string{"fuku.yml"},
			arguments: map[string]any{"config": "fuku.yml"},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "the root fuku.yml the call names beside a fuku.yaml",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend", "--config", filepath.Join(dir, "fuku.yml")).Return(stopped, nil),
				)
			},
			files:     []string{"fuku.yaml", "fuku.yml"},
			arguments: map[string]any{"config": "fuku.yml"},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name: "the root fuku.yaml the call names beside a fuku.yml",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(stopped, nil),
				)
			},
			files:     []string{"fuku.yaml", "fuku.yml"},
			arguments: map[string]any{"config": "fuku.yaml"},
			expected:  `{"exitCode":0,"summary":"stopped profile backend"}`,
		},
		{
			name:            "a missing config directory",
			before:          func() {},
			arguments:       map[string]any{"config": "missing/fuku.yaml"},
			expected:        unresolved,
			expectedIsError: true,
		},
		{
			name: "no running fuku",
			before: func() {
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			expected:        "no fuku instance is running in " + dir + "; call run first",
			expectedIsError: true,
		},
		{
			name: "fuku stop fails",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return(nil, errors.New("fuku stop backend failed: exit status 1: cannot reach the running fuku, nothing was stopped")),
				)
			},
			expected:        "fuku stop backend failed: exit status 1: cannot reach the running fuku, nothing was stopped",
			expectedIsError: true,
		},
		{
			name: "fuku stop fails after a long output",
			before: func() {
				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "backend").Return([]byte(noisy.String()), errors.New("fuku stop backend failed: exit status 1: cannot reach the running fuku, nothing was stopped")),
				)
			},
			expected:        tail.String(),
			expectedIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, file := range tt.files {
				path := filepath.Join(dir, file)
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				t.Cleanup(func() { require.NoError(t, os.Remove(path)) })
			}

			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolStop, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
		})
	}
}

func Test_Server_handleService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockExecutor := NewMockExecutor(ctrl)
	mockAPI := NewMockAPI(ctrl)
	mockCandidate := NewMockAPI(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	token := "secret-token"
	apiID := "0b4d5c1e-7f42-4f0a-9d4e-2f6b8a1c9e01"
	fingerprint := instance.Fingerprint(dir)
	announcedAddress := "127.0.0.1:3858"
	configuredAddress := "127.0.0.1:4000"
	refusedAddress := "10.1.2.3:3858"
	errNotLoopback := errors.New("the fuku REST API address is not a loopback address: " + refusedAddress)
	services := []model.Service{
		{ID: apiID, Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42}},
		{ID: "8c2e9a4b-1d3f-4e5a-b6c7-d8e9f0a1b2c3", Name: "db", Status: model.StatusRunning, Process: model.Process{PID: 43}},
	}

	frame := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}, API: announcedAddress}
	tailLines := failedLines
	stream := func(_ context.Context, handler logs.Handler) {
		handler.HandleLog(model.LogLine{Service: "api", Message: "panic: missing DATABASE_URL"})
		handler.HandleLog(model.LogLine{Service: "api", Message: "exit status 1"})
	}
	acts := func(act contracts.Action, settled model.Service, done bool, err error) {
		gomock.InOrder(
			mockAPI.EXPECT().Services(gomock.Any()).Return(services, nil),
			mockAPI.EXPECT().Act(gomock.Any(), apiID, act, serviceWait).Return(settled, done, err),
		)
	}

	var loaded model.Config

	dial := func(model.Instance) Client { return mockClient }
	dialAPI := func(address, dialedToken string) (API, error) {
		if address == refusedAddress {
			return nil, errNotLoopback
		}

		if dialedToken == "" {
			assert.Equal(t, configuredAddress, address)

			return mockCandidate, nil
		}

		assert.Contains(t, []string{announcedAddress, configuredAddress}, address)
		assert.Equal(t, token, dialedToken)

		return mockAPI, nil
	}
	load := func(string) model.Config { return loaded }
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(dial, dialAPI, load, mockExecutor, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	tests := []struct {
		name            string
		before          func()
		arguments       map[string]any
		expected        string
		expectedIsError bool
	}{
		{
			name: "a restart that settles running",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{ID: apiID, Name: "api", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 77}}, true, nil)
			},
			arguments: map[string]any{"name": "api", "action": "restart"},
			expected:  `{"name":"api","pid":77,"state":"running","watching":true}`,
		},
		{
			name: "the configured API whose liveness names this project",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Listen: configuredAddress, Token: token}}}

				mockClient.EXPECT().Status().Return(contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}}, nil)
				mockCandidate.EXPECT().Live(gomock.Any()).Return(model.Instance{Fingerprint: fingerprint}, nil)
				acts(contracts.ActionRestart, model.Service{ID: apiID, Name: "api", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 77}}, true, nil)
			},
			arguments: map[string]any{"name": "api", "action": "restart"},
			expected:  `{"name":"api","pid":77,"state":"running","watching":true}`,
		},
		{
			name: "a start that settles failed",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockAPI.EXPECT().Services(gomock.Any()).Return(services, nil),
					mockAPI.EXPECT().Act(gomock.Any(), apiID, contracts.ActionStart, serviceWait).Return(model.Service{ID: apiID, Name: "api", Status: model.StatusFailed, Error: "exit status 1"}, true, nil),
					mockClient.EXPECT().Connect().Return(nil),
					mockClient.EXPECT().Subscribe([]string{"api"}, model.ReplayOptions{Tail: &tailLines, NoFollow: true}).Return(nil),
					mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, handler logs.Handler) {
						deadline, bounded := ctx.Deadline()
						assert.True(t, bounded, "the log read must run under the call deadline")
						assert.False(t, deadline.After(time.Now().Add(serviceTimeout)), "the log read must end with the call")

						stream(ctx, handler)
					}).Return(nil),
					mockClient.EXPECT().Close().Return(nil),
				)
			},
			arguments: map[string]any{"name": "api", "action": "start"},
			expected:  `{"lines":["panic: missing DATABASE_URL","exit status 1"],"name":"api","pid":0,"state":"failed","watching":false}`,
		},
		{
			name: "a failed service whose logs cannot be read",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				gomock.InOrder(
					mockClient.EXPECT().Status().Return(frame, nil),
					mockAPI.EXPECT().Services(gomock.Any()).Return(services, nil),
					mockAPI.EXPECT().Act(gomock.Any(), apiID, contracts.ActionStart, serviceWait).Return(model.Service{ID: apiID, Name: "api", Status: model.StatusFailed}, true, nil),
					mockClient.EXPECT().Connect().Return(errors.New("connection refused")),
				)
			},
			arguments:       map[string]any{"name": "api", "action": "start"},
			expected:        "failed to start api: the service failed, and its logs cannot be read: connection refused",
			expectedIsError: true,
		},
		{
			name: "a restart the wait outlives",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{}, false, nil)
				mockAPI.EXPECT().Service(gomock.Any(), apiID).Return(model.Service{ID: apiID, Name: "api", Status: model.StatusRestarting, Watching: true}, nil)
			},
			arguments: map[string]any{"name": "api", "action": "restart"},
			expected:  `{"inProgress":true,"name":"api","pid":0,"state":"restarting","watching":true}`,
		},
		{
			name: "a waiting restart whose service the instance no longer has",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{}, false, nil)
				mockAPI.EXPECT().Service(gomock.Any(), apiID).Return(model.Service{}, fmt.Errorf("%w: service not found", contracts.ErrAPIRejected))
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "failed to restart api: fuku rejected the action: service not found; call status to see the service state",
			expectedIsError: true,
		},
		{
			name: "an unknown service name",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				mockAPI.EXPECT().Services(gomock.Any()).Return(services, nil)
			},
			arguments:       map[string]any{"name": "nope", "action": "restart"},
			expected:        "unknown service nope; the running profile has api, db",
			expectedIsError: true,
		},
		{
			name: "the API is off",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}}, nil)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        errAPIOff.Error(),
			expectedIsError: true,
		},
		{
			name: "a stop the instance rejects",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionStop, model.Service{}, false, fmt.Errorf("%w: service is not running", contracts.ErrAPIRejected))
			},
			arguments:       map[string]any{"name": "api", "action": "stop"},
			expected:        "failed to stop api: fuku rejected the action: service is not running; call status to see the service state",
			expectedIsError: true,
		},
		{
			name: "an overloaded instance",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{}, false, contracts.ErrAPIOverloaded)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "failed to restart api: the running fuku is overloaded; retry the action",
			expectedIsError: true,
		},
		{
			name: "the API rejects the token on the action",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{}, false, contracts.ErrAPIUnauthorized)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "failed to restart api: " + errAPIUnauthorized.Error(),
			expectedIsError: true,
		},
		{
			name: "the API rejects the token on the service list",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				mockAPI.EXPECT().Services(gomock.Any()).Return(nil, contracts.ErrAPIUnauthorized)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        errAPIUnauthorized.Error(),
			expectedIsError: true,
		},
		{
			name: "a connection the instance drops during the action",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(frame, nil)
				acts(contracts.ActionRestart, model.Service{}, false, fmt.Errorf("%w: EOF", contracts.ErrAPIUnreachable))
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "failed to restart api: the fuku REST API is off or unreachable: EOF",
			expectedIsError: true,
		},
		{
			name: "an announced address the client refuses",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockClient.EXPECT().Status().Return(contracts.LogStatus{API: refusedAddress}, nil)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        errNotLoopback.Error(),
			expectedIsError: true,
		},
		{
			name:            "a missing config directory",
			before:          func() {},
			arguments:       map[string]any{"name": "api", "action": "restart", "config": "missing/fuku.yaml"},
			expected:        "failed to resolve project directory: lstat " + filepath.Join(dir, "missing") + ": no such file or directory",
			expectedIsError: true,
		},
		{
			name: "an invalid config",
			before: func() {
				loaded = model.Config{Error: errors.New("services.api.dir is required")}

				mockClient.EXPECT().Status().Return(frame, nil)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "failed to load the config in " + dir + ": services.api.dir is required",
			expectedIsError: true,
		},
		{
			name: "no running fuku",
			before: func() {
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			arguments:       map[string]any{"name": "api", "action": "restart"},
			expected:        "no fuku instance is running in " + dir + "; call run first",
			expectedIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolService, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
			assert.NotContains(t, result.Content[0].(*sdk.TextContent).Text, token)
			assert.NotContains(t, result.Content[0].(*sdk.TextContent).Text, apiID)
		})
	}
}
