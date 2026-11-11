package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Call_Elapsed(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{name: "below a second", duration: 4*time.Millisecond + 300*time.Microsecond, expected: "4ms"},
		{name: "just under a second", duration: time.Second - time.Microsecond, expected: "999ms"},
		{name: "exactly a second", duration: time.Second, expected: "1.0s"},
		{name: "a second and more", duration: 2140 * time.Millisecond, expected: "2.1s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Call{Duration: tt.duration}.Elapsed())
		})
	}
}

func Test_Server_logCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)
	mockClient := NewMockClient(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)

	token := "secret-token"
	apiID := "0b4d5c1e-7f42-4f0a-9d4e-2f6b8a1c9e01"
	services := []model.Service{{ID: apiID, Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 42}}}

	frame := contracts.LogStatus{Profile: "default", Services: []string{"api"}, API: "127.0.0.1:3858"}
	private := filepath.Join(dir, "private.yaml")
	output := []byte("api | service-log-line\nfuku | stopping\n")
	exited := exec.Command("sh", "-c", "exit 3").Run()
	failed := func(args string) error {
		return fmt.Errorf("fuku %s --config %s failed: %w: service-log-line", args, private, exited)
	}
	newInstance := func(model.Instance) Instance { return mockInstance }
	newClient := func(string, string) (Client, error) { return mockClient, nil }
	load := func(string, string) model.Config {
		return model.Config{Project: model.Project{Server: model.Server{Token: token}}}
	}
	timeout := 5 * time.Second
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	var stderr bytes.Buffer

	subject, err := NewServer(newInstance, newClient, load, mockExecutor, NewPlain(&stderr))
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
			name: "a successful call",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json", "--config", filepath.Join(dir, "private.yaml"), "--", "local-only").Return([]byte(`{"version":"1.0.0"}`), nil)
			},
			tool:      toolDoctor,
			arguments: map[string]any{"config": "private.yaml", "profile": "local-only"},
			expected:  `^ ✔ doctor {25}\d+ms\n$`,
		},
		{
			name: "a service call with its name, action and settled state",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockClient.EXPECT().Services(gomock.Any()).Return(services, nil),
					mockClient.EXPECT().Act(gomock.Any(), apiID, contracts.ActionRestart, serviceWait).Return(model.Service{ID: apiID, Name: "api", Status: model.StatusRunning, Process: model.Process{PID: 77}}, true, nil),
				)
			},
			tool:      toolService,
			arguments: map[string]any{"config": "private.yaml", "name": "api", "action": "restart"},
			expected:  `^ ✔ service api restart  running   \d+ms\n$`,
		},
		{
			name: "a service call whose result holds the token keeps its settled state",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockClient.EXPECT().Services(gomock.Any()).Return(services, nil),
					mockClient.EXPECT().Act(gomock.Any(), apiID, contracts.ActionRestart, serviceWait).Return(model.Service{ID: apiID, Name: "api " + token, Status: model.StatusRunning}, true, nil),
				)
			},
			tool:      toolService,
			arguments: map[string]any{"config": "private.yaml", "name": "api", "action": "restart"},
			expected:  `^ ✔ service api restart  running   \d+ms\n$`,
		},
		{
			name: "a service call whose target holds the token",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockClient.EXPECT().Services(gomock.Any()).Return(services, nil),
				)
			},
			tool:      toolService,
			arguments: map[string]any{"config": "private.yaml", "name": token, "action": "restart"},
			expected:  `^ ✘ service \[redacted\] restart .+ · \d+ms\n$`,
		},
		{
			name: "a service call that is still in progress",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockClient.EXPECT().Services(gomock.Any()).Return(services, nil),
					mockClient.EXPECT().Act(gomock.Any(), apiID, contracts.ActionStart, serviceWait).Return(model.Service{}, false, nil),
					mockClient.EXPECT().Service(gomock.Any(), apiID).Return(model.Service{ID: apiID, Name: "api", Status: model.StatusStarting}, nil),
				)
			},
			tool:      toolService,
			arguments: map[string]any{"config": "private.yaml", "name": "api", "action": "start"},
			expected:  `^ ✔ service api start    starting · in progress \d+ms\n$`,
		},
		{
			name: "a failed call",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:      toolLogs,
			arguments: map[string]any{"config": "private.yaml", "services": []string{"hidden-svc"}},
			expected:  `^ ✘ logs {27}no fuku instance is running in .+; call run first · \d+ms\n$`,
		},
		{
			name: "a failed call with a multi-line error",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, errors.New("first\x1b[31m\nsecond"))
			},
			tool:     toolLogs,
			expected: `^ ✘ logs {27}failed to read the status of fuku in .+: first\[31m · \d+ms\n$`,
		},
		{
			name: "a failed call whose error holds the token",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, errors.New("auth "+token))
			},
			tool:     toolLogs,
			expected: `^ ✘ logs {27}failed to read the status of fuku in .+: auth \[redacted\] · \d+ms\n$`,
		},
		{
			name:      "a call with an argument of the wrong type",
			before:    func() {},
			tool:      toolLogs,
			arguments: map[string]any{"services": "secret-arg-value"},
			expected:  `^ ✘ logs {27}invalid arguments · \d+ms\n$`,
		},
		{
			name: "a failed run",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "run", "--detached", "--config", private, "--", "local-only").Return(output, failed("run local-only --detached"))
			},
			tool:      toolRun,
			arguments: map[string]any{"config": "private.yaml", "profile": "local-only"},
			expected:  `^ ✘ run {28}fuku run failed · exit 3 · \d+ms\n$`,
		},
		{
			name: "a failed stop",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Status().Return(frame, nil),
					mockExecutor.EXPECT().Execute(gomock.Any(), dir, "stop", "--config", private, "--", "default").Return(output, failed("stop default")),
				)
			},
			tool:      toolStop,
			arguments: map[string]any{"config": "private.yaml"},
			expected:  `^ ✘ stop {27}fuku stop failed · exit 3 · \d+ms\n$`,
		},
		{
			name: "a failed doctor",
			before: func() {
				mockExecutor.EXPECT().Execute(gomock.Any(), dir, "doctor", "--json", "--config", private).Return(output, failed("doctor --json"))
			},
			tool:      toolDoctor,
			arguments: map[string]any{"config": "private.yaml"},
			expected:  `^ ✘ doctor {25}fuku doctor failed · exit 3 · \d+ms\n$`,
		},
		{
			name: "a failed command tool without an exit code",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:      toolStop,
			arguments: map[string]any{"config": "private.yaml"},
			expected:  `^ ✘ stop {27}fuku stop failed · \d+ms\n$`,
		},
		{
			name:     "an unknown tool",
			before:   func() {},
			tool:     "missing",
			expected: `^ ✘ missing {24}.+ · \d+ms\n$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			stderr.Reset()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			session.CallTool(ctx, &sdk.CallToolParams{Name: tt.tool, Arguments: tt.arguments})

			assert.Regexp(t, tt.expected, stderr.String())

			for _, secret := range []string{token, "private.yaml", "local-only", "hidden-svc", "service-log-line", "stopping", "second", "secret-arg-value"} {
				assert.NotContains(t, stderr.String(), secret)
			}
		})
	}
}

func Test_Server_logCalls_Concurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning).Times(2)

	newInstance := func(model.Instance) Instance { return mockInstance }

	var newClient func(string, string) (Client, error)

	load := func(string, string) model.Config { return model.Config{} }
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	var stderr bytes.Buffer

	subject, err := NewServer(newInstance, newClient, load, NewBinary(), NewPlain(&stderr))
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	var wg sync.WaitGroup

	for range 2 {
		wg.Go(func() {
			_, err := session.CallTool(t.Context(), &sdk.CallToolParams{Name: toolLogs})
			assert.NoError(t, err)
		})
	}

	wg.Wait()

	assert.Regexp(t, `^( ✘ logs {27}no fuku instance is running in .+; call run first · \d+ms\n){2}$`, stderr.String())
}

func Test_bannerRecord(t *testing.T) {
	identity := model.Instance{Project: "/work/shop", Fingerprint: instance.Fingerprint("/work/shop")}
	protocol := sdk.SupportedProtocolVersions()[0]
	tools := []string{"config", "doctor", "logs", "run", "service", "status", "stop"}

	tests := []struct {
		name     string
		status   contracts.LogStatus
		err      error
		expected Banner
	}{
		{
			name:   "running with an API",
			status: contracts.LogStatus{Profile: "backend", API: "127.0.0.1:9876", PID: 4242},
			expected: Banner{
				Protocol:    protocol,
				Project:     "/work/shop",
				Fingerprint: identity.Fingerprint,
				State:       InstanceStateRunning,
				Detail:      "profile backend · http://127.0.0.1:9876 · pid 4242",
				Tools:       tools,
			},
		},
		{
			name:   "running without an API",
			status: contracts.LogStatus{Profile: "backend", PID: 4242},
			expected: Banner{
				Protocol:    protocol,
				Project:     "/work/shop",
				Fingerprint: identity.Fingerprint,
				State:       InstanceStateRunning,
				Detail:      "profile backend · pid 4242",
				Tools:       tools,
			},
		},
		{
			name: "not running",
			err:  fmt.Errorf("wrapped: %w", contracts.ErrNoInstanceRunning),
			expected: Banner{
				Protocol:    protocol,
				Project:     "/work/shop",
				Fingerprint: identity.Fingerprint,
				State:       InstanceStateNotRunning,
				Detail:      "call run to start",
				Tools:       tools,
			},
		},
		{
			name: "unknown",
			err:  errors.New("failed to read the status: connection reset\nby peer"),
			expected: Banner{
				Protocol:    protocol,
				Project:     "/work/shop",
				Fingerprint: identity.Fingerprint,
				State:       InstanceStateUnknown,
				Detail:      "failed to read the status: connection reset",
				Tools:       tools,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, bannerRecord(identity, tt.status, tt.err))
		})
	}
}

func Test_failure(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		called   *sdk.CallToolResult
		err      error
		expected string
	}{
		{
			name:     "a protocol error",
			tool:     toolLogs,
			err:      errors.New("unknown tool\nsecond"),
			expected: "unknown tool",
		},
		{
			name:     "a call that succeeded",
			tool:     toolLogs,
			called:   &sdk.CallToolResult{},
			expected: "",
		},
		{
			name:     "a failed tool with no text",
			tool:     toolLogs,
			called:   &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.ImageContent{}}},
			expected: "the tool failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, failure(tt.tool, tt.called, tt.err))
		})
	}
}

func Test_oneLine(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "a plain line", text: "unknown service nope", expected: "unknown service nope"},
		{name: "the first of several lines", text: "first\nsecond", expected: "first"},
		{name: "control characters", text: "red \x1b[31mtext\r\t", expected: "red [31mtext"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, oneLine(tt.text))
		})
	}
}
