package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/app/logs"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Server_CallTool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	identity := model.Instance{Project: dir, Fingerprint: instance.Fingerprint(dir)}
	newInstance := func(dialed model.Instance) Instance {
		assert.Equal(t, identity, dialed)

		return mockInstance
	}

	var newClient func(string, string) (Client, error)

	load := func(string, string) model.Config { return model.Config{} }
	notRunning := "no fuku instance is running in " + dir + "; call run first"
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	tests := []struct {
		name      string
		before    func()
		tool      string
		arguments map[string]any
		expected  string
	}{
		{
			name: "logs without an instance",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:      toolLogs,
			arguments: map[string]any{"services": []string{"api"}, "tail": 10},
			expected:  notRunning,
		},
		{
			name: "stop without an instance",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:     toolStop,
			expected: notRunning,
		},
		{
			name: "service without an instance",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:      toolService,
			arguments: map[string]any{"name": "api", "action": "restart"},
			expected:  notRunning,
		},
		{
			name: "status with an unreadable instance",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, errors.New("timeout"))
			},
			tool:     toolStatus,
			expected: "failed to read the status of fuku in " + dir + ": timeout",
		},
		{
			name:      "status with a missing config directory",
			before:    func() {},
			tool:      toolStatus,
			arguments: map[string]any{"config": "missing/fuku.yaml"},
			expected:  instance.ErrFailedToResolveProject.Error(),
		},
		{
			name:     "run without a profile",
			before:   func() {},
			tool:     toolRun,
			expected: "profile",
		},
		{
			name:      "run with an empty profile",
			before:    func() {},
			tool:      toolRun,
			arguments: map[string]any{"profile": ""},
			expected:  "minLength",
		},
		{
			name:      "service with an empty name",
			before:    func() {},
			tool:      toolService,
			arguments: map[string]any{"name": "", "action": "restart"},
			expected:  "minLength",
		},
		{
			name:      "service with an unknown action",
			before:    func() {},
			tool:      toolService,
			arguments: map[string]any{"name": "api", "action": "kill"},
			expected:  "action",
		},
		{
			name:      "logs with a tail below one",
			before:    func() {},
			tool:      toolLogs,
			arguments: map[string]any{"tail": 0},
			expected:  "tail",
		},
		{
			name:      "logs with a tail above the cap",
			before:    func() {},
			tool:      toolLogs,
			arguments: map[string]any{"tail": 1001},
			expected:  "tail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tt.tool, Arguments: tt.arguments})

			require.NoError(t, err)
			assert.True(t, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].(*sdk.TextContent).Text, tt.expected)
		})
	}
}

func Test_add_InvalidInput(t *testing.T) {
	server := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	handler := func(context.Context, *sdk.CallToolRequest, struct{ Callback func() }) (*sdk.CallToolResult, commandOutput, error) {
		return nil, commandOutput{}, nil
	}

	err := add(server, "broken", "a tool whose input has no schema", nil, handler)

	assert.ErrorContains(t, err, "failed to infer the input schema of broken")
}

func Test_Server_handleStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)
	mockClient := NewMockClient(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	token := "secret-token"
	fingerprint := instance.Fingerprint(dir)
	announcedAddress := "127.0.0.1:3858"
	configuredAddress := "127.0.0.1:4000"
	refusedAddress := "10.1.2.3:3858"
	errNotLoopback := errors.New("the fuku REST API address is not a loopback address: " + refusedAddress)

	snapshot := model.Snapshot{Profile: "default", Phase: model.PhaseRunning}
	services := []model.Service{
		{ID: "u-1", Name: "api", Status: model.StatusRunning, Watching: true, Process: model.Process{PID: 42, StartedAt: time.Now().Add(-7*time.Second - 500*time.Millisecond)}},
		{ID: "u-2", Name: "db", Status: model.StatusFailed},
	}

	frame := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}, Mode: model.RunModeDetached}
	announced := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}, API: announcedAddress, Mode: model.RunModeDetached}
	older := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}, API: announcedAddress}
	states := `{"mode":"detached","phase":"running","profile":"default","running":true,"services":[{"name":"api","pid":42,"state":"running","uptime":7,"watching":true},{"name":"db","pid":0,"state":"failed","uptime":0,"watching":false}]}`
	unknown := `{"mode":"detached","note":"service states need the fuku REST API, which is off or unreachable","phase":"unknown","profile":"default","running":true,"services":[{"name":"api","pid":0,"state":"unknown","uptime":0,"watching":false},{"name":"db","pid":0,"state":"unknown","uptime":0,"watching":false}]}`

	var (
		loaded      model.Config
		loadedPaths []string
	)

	newInstance := func(model.Instance) Instance { return mockInstance }
	newClient := func(address, dialedToken string) (Client, error) {
		if address == refusedAddress {
			return nil, errNotLoopback
		}

		assert.Contains(t, []string{"", token}, dialedToken)

		return mockClient, nil
	}
	load := func(project, path string) model.Config {
		assert.Equal(t, dir, project)

		loadedPaths = append(loadedPaths, path)

		return loaded
	}
	readsState := func() {
		gomock.InOrder(
			mockClient.EXPECT().Status(gomock.Any()).Return(snapshot, nil),
			mockClient.EXPECT().Services(gomock.Any()).Return(services, nil),
		)
	}
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
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
		expectedPath    string
		expectedIsError bool
	}{
		{
			name: "the API the status frame announces",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				readsState()
			},
			expected: states,
		},
		{
			name: "an older fuku that sends no mode",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(older, nil)
				readsState()
			},
			expected: strings.Replace(states, `"mode":"detached"`, `"mode":"unknown"`, 1),
		},
		{
			name: "a config file the call names",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				readsState()
			},
			arguments:    map[string]any{"config": "custom.yaml"},
			expected:     states,
			expectedPath: "custom.yaml",
		},
		{
			name: "the root fuku.yaml the call names",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				readsState()
			},
			arguments:    map[string]any{"config": "fuku.yaml"},
			expected:     states,
			expectedPath: "fuku.yaml",
		},
		{
			name: "the configured API whose liveness names this project",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Listen: configuredAddress, Token: token}}}

				mockInstance.EXPECT().Status().Return(frame, nil)
				mockClient.EXPECT().Live(gomock.Any()).Return(model.Instance{Fingerprint: fingerprint}, nil)
				readsState()
			},
			expected: states,
		},
		{
			name: "the configured API of another project",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Listen: configuredAddress, Token: token}}}

				mockInstance.EXPECT().Status().Return(frame, nil)
				mockClient.EXPECT().Live(gomock.Any()).Return(model.Instance{Fingerprint: "another-project"}, nil)
			},
			expected: unknown,
		},
		{
			name: "no listener on the configured API",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Listen: configuredAddress, Token: token}}}

				mockInstance.EXPECT().Status().Return(frame, nil)
				mockClient.EXPECT().Live(gomock.Any()).Return(model.Instance{}, contracts.ErrAPIUnreachable)
			},
			expected: unknown,
		},
		{
			name: "the API is off",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(frame, nil)
			},
			expected: unknown,
		},
		{
			name: "the announced API stops answering",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				mockClient.EXPECT().Status(gomock.Any()).Return(model.Snapshot{}, fmt.Errorf("%w: connection refused", contracts.ErrAPIUnreachable))
			},
			expected: unknown,
		},
		{
			name: "the API rejects the token",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				mockClient.EXPECT().Status(gomock.Any()).Return(model.Snapshot{}, contracts.ErrAPIUnauthorized)
			},
			expected:        errAPIUnauthorized.Error(),
			expectedIsError: true,
		},
		{
			name: "the API fails",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				mockClient.EXPECT().Status(gomock.Any()).Return(model.Snapshot{}, errors.New("the fuku REST API failed: /api/v1/status answered 500 Internal Server Error"))
			},
			expected:        "the fuku REST API failed: /api/v1/status answered 500 Internal Server Error",
			expectedIsError: true,
		},
		{
			name: "the API fails on the services",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(announced, nil)
				gomock.InOrder(
					mockClient.EXPECT().Status(gomock.Any()).Return(snapshot, nil),
					mockClient.EXPECT().Services(gomock.Any()).Return(nil, errors.New("the fuku REST API failed: /api/v1/services answered 500 Internal Server Error")),
				)
			},
			expected:        "the fuku REST API failed: /api/v1/services answered 500 Internal Server Error",
			expectedIsError: true,
		},
		{
			name: "an announced address the client refuses",
			before: func() {
				loaded = model.Config{Project: model.Project{Server: model.Server{Token: token}}}

				mockInstance.EXPECT().Status().Return(contracts.LogStatus{API: refusedAddress}, nil)
			},
			expected:        errNotLoopback.Error(),
			expectedIsError: true,
		},
		{
			name: "an invalid config",
			before: func() {
				loaded = model.Config{Error: errors.New("services.api.dir is required\nservices.db.dir is required")}

				mockInstance.EXPECT().Status().Return(announced, nil)
			},
			expected: `{"mode":"detached","note":"failed to load the config in ` + dir + `: services.api.dir is required","phase":"unknown","profile":"default","running":true,"services":[{"name":"api","pid":0,"state":"unknown","uptime":0,"watching":false},{"name":"db","pid":0,"state":"unknown","uptime":0,"watching":false}]}`,
		},
		{
			name: "an invalid config without an instance",
			before: func() {
				loaded = model.Config{Error: errors.New("services.api.dir is required")}

				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			expected: `{"mode":"","note":"no fuku runs in ` + dir + `; call run first","phase":"","profile":"","running":false,"services":[]}`,
		},
		{
			name: "the profiles of the config without an instance",
			before: func() {
				loaded = model.Config{Project: model.Project{Profiles: map[string]model.Profile{"default": {All: true}, "core": {Services: []string{"api"}}}}}

				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			arguments:    map[string]any{"config": "custom.yaml"},
			expected:     `{"mode":"","note":"no fuku runs in ` + dir + `; call run with one of: core, default","phase":"","profile":"","profiles":["core","default"],"running":false,"services":[]}`,
			expectedPath: "custom.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadedPaths = loadedPaths[:0]

			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolStatus, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedPath, append(loadedPaths, "")[0])
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
			assert.NotContains(t, result.Content[0].(*sdk.TextContent).Text, token)
		})
	}
}

func Test_Server_handleLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	var loaded model.Config

	frame := contracts.LogStatus{Profile: "default", Services: []string{"api", "db"}}
	tailDefaultLines := tailDefault
	tailFive := 5
	stream := func(_ context.Context, handler logs.Handler) {
		require.NoError(t, handler.HandleStatus(frame))
		handler.HandleLog(model.LogLine{Service: "api", Message: "listening on :8080"})
		handler.HandleLog(model.LogLine{Service: "db", Message: "ready"})
	}
	newInstance := func(model.Instance) Instance { return mockInstance }

	var newClient func(string, string) (Client, error)

	load := func(string, string) model.Config { return loaded }
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
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
			name: "every service with the default tail",
			before: func() {
				loaded = model.Config{}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe(nil, model.ReplayOptions{Tail: &tailDefaultLines, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Do(stream).Return(nil),
					mockInstance.EXPECT().Close().Return(nil),
				)
			},
			expected: `{"lines":[{"message":"listening on :8080","service":"api"},{"message":"ready","service":"db"}]}`,
		},
		{
			name: "the named services with a tail",
			before: func() {
				loaded = model.Config{}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe([]string{"api"}, model.ReplayOptions{Tail: &tailFive, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil),
					mockInstance.EXPECT().Close().Return(nil),
				)
			},
			arguments: map[string]any{"services": []string{"api"}, "tail": 5},
			expected:  `{"lines":[]}`,
		},
		{
			name:            "a missing config directory",
			before:          func() {},
			arguments:       map[string]any{"config": "missing/fuku.yaml"},
			expected:        "failed to resolve project directory: lstat " + filepath.Join(dir, "missing") + ": no such file or directory",
			expectedIsError: true,
		},
		{
			name: "an unknown service name",
			before: func() {
				loaded = model.Config{}

				mockInstance.EXPECT().Status().Return(frame, nil)
			},
			arguments:       map[string]any{"services": []string{"api", "web", "cache"}},
			expected:        "unknown service web, cache; the running profile has api, db",
			expectedIsError: true,
		},
		{
			name: "the instance cannot be dialed",
			before: func() {
				loaded = model.Config{}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(errors.New("connection refused")),
				)
			},
			expected:        "failed to read the logs of fuku in " + dir + ": connection refused",
			expectedIsError: true,
		},
		{
			name: "the subscription cannot be sent",
			before: func() {
				loaded = model.Config{}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe(nil, model.ReplayOptions{Tail: &tailDefaultLines, NoFollow: true}).Return(errors.New("broken pipe")),
					mockInstance.EXPECT().Close().Return(nil),
				)
			},
			expected:        "failed to read the logs of fuku in " + dir + ": broken pipe",
			expectedIsError: true,
		},
		{
			name: "an instance without bounded reads",
			before: func() {
				loaded = model.Config{}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe(nil, model.ReplayOptions{Tail: &tailDefaultLines, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(contracts.ErrBoundedReadNotSupported),
					mockInstance.EXPECT().Close().Return(nil),
				)
			},
			expected:        "failed to read the logs of fuku in " + dir + ": " + contracts.ErrBoundedReadNotSupported.Error(),
			expectedIsError: true,
		},
		{
			name: "an invalid config",
			before: func() {
				loaded = model.Config{Error: errors.New("services.api.dir is required")}

				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe(nil, model.ReplayOptions{Tail: &tailDefaultLines, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Do(stream).Return(nil),
					mockInstance.EXPECT().Close().Return(nil),
				)
			},
			expected: `{"lines":[{"message":"listening on :8080","service":"api"},{"message":"ready","service":"db"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolLogs, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
		})
	}
}

func Test_Server_handleDoctor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	report := []byte(`{"overallStatus":"fail","tally":{"fail":1}}`)
	newInstance := func(model.Instance) Instance { return mockInstance }

	var newClient func(string, string) (Client, error)

	load := func(string, string) model.Config { return model.Config{Error: errors.New("broken config")} }
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
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
			name: "the default profile and config",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json").Return([]byte(`{"overallStatus":"ok"}`), nil)
			},
			expected: `{"overallStatus":"ok"}`,
		},
		{
			name: "a profile and a config file",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json", "--config", filepath.Join(dir, "custom.yaml"), "--", "backend").Return([]byte(`{"overallStatus":"ok"}`), nil)
			},
			arguments: map[string]any{"profile": "backend", "config": "custom.yaml"},
			expected:  `{"overallStatus":"ok"}`,
		},
		{
			name: "a failing report",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json").Return(report, errors.New("exit status 1"))
			},
			expected: `{"overallStatus":"fail","tally":{"fail":1}}`,
		},
		{
			name: "a binary that prints no report",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json").Return(nil, errors.New("fuku doctor --json failed: exit status 2: unknown flag"))
			},
			expected:        "failed to read the doctor report: fuku doctor --json failed: exit status 2: unknown flag\nunexpected end of JSON input",
			expectedIsError: true,
		},
		{
			name:            "a missing config directory",
			before:          func() {},
			arguments:       map[string]any{"config": "missing/fuku.yaml"},
			expected:        instance.ErrFailedToResolveProject.Error() + ": lstat " + filepath.Join(dir, "missing") + ": no such file or directory",
			expectedIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolDoctor, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
		})
	}
}

func Test_Server_handleConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	token := "secret-token"
	project := model.Project{
		Services: []model.Service{
			{
				Name:        "db",
				Command:     "make run DB_TOKEN=secret-token",
				Directory:   filepath.Join(dir, "db"),
				Tier:        "foundation",
				Readiness:   &model.Readiness{Type: model.ReadinessTCP, Address: "127.0.0.1:5432", Timeout: 30 * time.Second},
				Environment: &model.EnvFiles{Files: []string{".env"}},
			},
			{
				Name:      "api",
				Command:   "go run .",
				Directory: "api",
				Tier:      "platform",
				Readiness: &model.Readiness{Type: model.ReadinessHTTP, URL: "http://127.0.0.1:8080/health?token=secret-token", Timeout: 5 * time.Second},
				Watch: &model.Watch{
					Include:  []string{"**/*.go"},
					Ignore:   []string{"**/*_test.go"},
					Shared:   []string{"pkg/common"},
					Debounce: 300 * time.Millisecond,
				},
			},
			{
				Name:      "worker",
				Command:   "make run",
				Directory: "/elsewhere/worker",
				Tier:      "platform",
				Readiness: &model.Readiness{Type: model.ReadinessLog, Pattern: "ready", Timeout: time.Minute},
			},
		},
		Profiles: map[string]model.Profile{"default": {All: true}, "core": {Services: []string{"worker", "db"}}},
		Exclude:  []string{"worker"},
		Server:   model.Server{Listen: "127.0.0.1:3858", Token: token},
	}
	db := `{"command":"make run DB_TOKEN=[redacted]","dir":"db","envFiles":[".env"],"name":"db","readiness":{"target":"127.0.0.1:5432","timeout":"30s","type":"tcp"},"tier":"foundation"}`
	api := `{"command":"go run .","dir":"api","name":"api","readiness":{"target":"http://127.0.0.1:8080/health?token=[redacted]","timeout":"5s","type":"http"},"tier":"platform","watch":{"debounce":"300ms","ignore":["**/*_test.go"],"include":["**/*.go"],"shared":["pkg/common"]}}`
	worker := `{"command":"make run","dir":"/elsewhere/worker","name":"worker","readiness":{"target":"ready","timeout":"1m0s","type":"log"},"tier":"platform"}`
	head := `{"api":{"enabled":true,"listen":"127.0.0.1:3858"},"profiles":{"core":["db"],"default":["db","api"]},"services":`

	var (
		loaded      model.Config
		loadedPaths []string
	)

	newInstance := func(model.Instance) Instance {
		t.Error("config dials no instance")

		return mockInstance
	}

	var newClient func(string, string) (Client, error)

	load := func(project, path string) model.Config {
		assert.Equal(t, dir, project)

		loadedPaths = append(loadedPaths, path)

		return loaded
	}
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	tests := []struct {
		name            string
		loaded          model.Config
		arguments       map[string]any
		expected        string
		expectedPath    string
		expectedIsError bool
	}{
		{
			name:     "every service",
			loaded:   model.Config{Project: project},
			expected: head + `[` + db + `,` + api + `,` + worker + `]}`,
		},
		{
			name:      "one service",
			loaded:    model.Config{Project: project},
			arguments: map[string]any{"service": "api"},
			expected:  head + `[` + api + `]}`,
		},
		{
			name:         "a config file the call names with the API off",
			loaded:       model.Config{Project: model.Project{Services: []model.Service{}, Profiles: map[string]model.Profile{}}},
			arguments:    map[string]any{"config": "custom.yaml"},
			expected:     `{"api":{"enabled":false,"listen":""},"profiles":{},"services":[]}`,
			expectedPath: "custom.yaml",
		},
		{
			name: "a readiness type the config does not know",
			loaded: model.Config{Project: model.Project{
				Services: []model.Service{{Name: "cache", Command: "make run", Directory: "cache", Readiness: &model.Readiness{Type: "grpc", Address: "127.0.0.1:9000", URL: "http://127.0.0.1:9000", Pattern: "ready", Timeout: time.Second}}},
				Profiles: map[string]model.Profile{},
			}},
			expected: `{"api":{"enabled":false,"listen":""},"profiles":{},"services":[{"command":"make run","dir":"cache","name":"cache","readiness":{"target":"","timeout":"1s","type":"grpc"},"tier":""}]}`,
		},
		{
			name:            "an unknown service",
			loaded:          model.Config{Project: project},
			arguments:       map[string]any{"service": "nope"},
			expected:        "unknown service nope; the config has db, api, worker",
			expectedIsError: true,
		},
		{
			name:            "a profile with a service the config lacks",
			loaded:          model.Config{Project: model.Project{Services: []model.Service{}, Profiles: map[string]model.Profile{"broken": {Services: []string{"ghost"}}}}},
			expected:        "failed to resolve profile broken: service not found: 'ghost'",
			expectedIsError: true,
		},
		{
			name:            "an invalid config",
			loaded:          model.Config{Error: errors.New("services.api.dir is required")},
			expected:        "failed to load the config in " + dir + ": services.api.dir is required",
			expectedIsError: true,
		},
		{
			name:            "a missing config directory",
			arguments:       map[string]any{"config": "missing/fuku.yaml"},
			expected:        instance.ErrFailedToResolveProject.Error() + ": lstat " + filepath.Join(dir, "missing") + ": no such file or directory",
			expectedIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loaded, loadedPaths = tt.loaded, loadedPaths[:0]

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: toolConfig, Arguments: tt.arguments})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tt.expectedPath, append(loadedPaths, "")[0])
			assert.Equal(t, tt.expectedIsError, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
			assert.NotContains(t, result.Content[0].(*sdk.TextContent).Text, token)
		})
	}
}

func Test_Server_read(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	identity := model.Instance{Project: "/projects/shop", Fingerprint: "shop"}
	tail := 5
	expired, cancel := context.WithCancel(t.Context())
	cancel()

	stream := func(_ context.Context, handler logs.Handler) {
		handler.HandleLog(model.LogLine{Service: "api", Message: "listening on :8080"})
	}
	newInstance := func(model.Instance) Instance { return mockInstance }

	var newClient func(string, string) (Client, error)

	load := func(string, string) model.Config { return model.Config{} }

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(io.Discard))
	require.NoError(t, err)

	tests := []struct {
		name     string
		before   func() context.Context
		expected logsOutput
	}{
		{
			name: "the instance ends the stream",
			before: func() context.Context {
				gomock.InOrder(
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe([]string{"api"}, model.ReplayOptions{Tail: &tail, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Do(stream).Return(nil),
					mockInstance.EXPECT().Close().Return(nil),
				)

				return t.Context()
			},
			expected: logsOutput{Lines: []logLine{{Service: "api", Message: "listening on :8080"}}},
		},
		{
			name: "the read times out",
			before: func() context.Context {
				gomock.InOrder(
					mockInstance.EXPECT().Connect().Return(nil),
					mockInstance.EXPECT().Subscribe([]string{"api"}, model.ReplayOptions{Tail: &tail, NoFollow: true}).Return(nil),
					mockInstance.EXPECT().Stream(gomock.Any(), gomock.Any()).Do(stream).Return(nil),
					mockInstance.EXPECT().Close().Return(nil),
				)

				return expired
			},
			expected: logsOutput{Lines: []logLine{{Service: "api", Message: "listening on :8080"}}, Truncated: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			result, err := subject.read(ctx, identity, []string{"api"}, tail)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
