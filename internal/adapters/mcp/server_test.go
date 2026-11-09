package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewServer(t *testing.T) {
	dial := func(model.Instance) Client { return nil }

	var dialAPI APIDialer

	load := func(string) model.Config { return model.Config{} }
	execute := NewBinary()
	readOnly := &sdk.ToolAnnotations{ReadOnlyHint: true}
	expectedNames := []string{toolDoctor, toolLogs, toolRun, toolService, toolStatus, toolStop}
	expectedAnnotations := map[string]*sdk.ToolAnnotations{
		toolStatus:  readOnly,
		toolLogs:    readOnly,
		toolDoctor:  readOnly,
		toolRun:     {DestructiveHint: new(false)},
		toolStop:    {DestructiveHint: new(true)},
		toolService: {DestructiveHint: new(false)},
	}
	clientTransport, serverTransport := sdk.NewInMemoryTransports()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)

	subject, err := NewServer(dial, dialAPI, load, execute, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	_, err = subject.server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)

	defer session.Close()

	result, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)

	names := make([]string, 0, len(result.Tools))
	annotations := map[string]*sdk.ToolAnnotations{}
	descriptions := map[string]string{}
	configured := map[string]bool{}

	for _, tool := range result.Tools {
		names = append(names, tool.Name)
		annotations[tool.Name] = tool.Annotations
		descriptions[tool.Name] = tool.Description
		configured[tool.Name] = tool.InputSchema.(map[string]any)["properties"].(map[string]any)["config"] != nil
	}

	assert.Equal(t, expectedNames, names)
	assert.Equal(t, expectedAnnotations, annotations)
	assert.Equal(t, map[string]bool{toolDoctor: true, toolLogs: true, toolRun: true, toolService: true, toolStatus: true, toolStop: true}, configured)
	assert.Equal(t, instructions, session.InitializeResult().Instructions)
	assert.LessOrEqual(t, len(instructions), 512)
	assert.Contains(t, instructions, "Never read fuku.yaml or fuku.override.yaml whole; they may hold secrets.")
	assert.Contains(t, instructions, "Call doctor only after a failed run or a config edit, never before a plain run.")
	assert.Contains(t, instructions, "Omit config unless the user names a config file other than fuku.yaml.")
	assert.Contains(t, instructions, "run only when nothing runs, stop only when the user asks.")
	assert.Contains(t, descriptions[toolStatus], "its mode")
	assert.Contains(t, descriptions[toolStatus], noteMode)
	assert.Contains(t, noteMode, "do not stop it to apply a config change, tell the developer the change needs their restart")
	assert.Contains(t, noteWatching, "shows a new PID or failed, then test it; restart only a service that is not watching")
	assert.Contains(t, descriptions[toolStatus], noteWatching)
	assert.Contains(t, descriptions[toolService], noteWatching)
	assert.Contains(t, descriptions[toolStop], "leave fuku running after the task; stop only when the user asks, or to apply a config change when status reports mode detached")
}

func Test_Server_Run(t *testing.T) {
	t.Setenv(envProjectDir, "")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	dial := func(model.Instance) Client { return mockClient }

	var dialAPI APIDialer

	load := func(string) model.Config { return model.Config{} }
	execute := NewBinary()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	subject, err := NewServer(dial, dialAPI, load, execute, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	tests := []struct {
		name          string
		before        func(t *testing.T) (*os.File, context.Context)
		expectedExit  int
		expectedError require.ErrorAssertionFunc
	}{
		{
			name: "stdin closes",
			before: func(t *testing.T) (*os.File, context.Context) {
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)

				reader, writer, err := os.Pipe()
				require.NoError(t, err)
				require.NoError(t, writer.Close())

				return reader, t.Context()
			},
			expectedError: require.NoError,
		},
		{
			name: "the run is cancelled",
			before: func(t *testing.T) (*os.File, context.Context) {
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)

				reader, writer, err := os.Pipe()
				require.NoError(t, err)
				t.Cleanup(func() { writer.Close() })

				return reader, cancelled
			},
			expectedError: require.NoError,
		},
		{
			name: "stdin cannot be read",
			before: func(t *testing.T) (*os.File, context.Context) {
				mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)

				dir, err := os.Open(t.TempDir())
				require.NoError(t, err)

				return dir, t.Context()
			},
			expectedExit:  1,
			expectedError: require.Error,
		},
		{
			name: "the project root is missing",
			before: func(t *testing.T) (*os.File, context.Context) {
				t.Setenv(envProjectDir, filepath.Join(t.TempDir(), "missing"))

				return os.Stdin, t.Context()
			},
			expectedExit:  1,
			expectedError: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin := os.Stdin

			t.Cleanup(func() { os.Stdin = stdin })

			reader, ctx := tt.before(t)
			os.Stdin = reader

			exitCode, err := subject.Run(ctx)

			tt.expectedError(t, err)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func Test_Server_Run_EntersProjectRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, root)
	t.Chdir(t.TempDir())

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer writer.Close()

	stdin := os.Stdin
	os.Stdin = reader

	t.Cleanup(func() { os.Stdin = stdin })

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)

	dial := func(model.Instance) Client { return mockClient }

	var dialAPI APIDialer

	load := func(string) model.Config { return model.Config{} }
	execute := NewBinary()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	subject, err := NewServer(dial, dialAPI, load, execute, terminal.NewTheme(terminal.AppearanceDark), io.Discard)
	require.NoError(t, err)

	exitCode, err := subject.Run(cancelled)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, root, wd)
}

func Test_Server_Run_WritesOnlyProtocolToStdout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, dir)
	t.Chdir(t.TempDir())

	stdinReader, stdinWriter, err := os.Pipe()
	require.NoError(t, err)

	defer stdinWriter.Close()

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinReader, stdoutWriter

	t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })

	mockClient.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning).Times(2)

	dial := func(model.Instance) Client { return mockClient }

	var dialAPI APIDialer

	load := func(string) model.Config { return model.Config{} }

	var stderr bytes.Buffer

	subject, err := NewServer(dial, dialAPI, load, NewBinary(), terminal.NewTheme(terminal.AppearanceDark), &stderr)
	require.NoError(t, err)

	done := make(chan error, 1)

	go func() {
		_, err := subject.Run(t.Context())

		stdoutWriter.Close()

		done <- err
	}()

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"status","arguments":{}}}`,
	}

	_, err = stdinWriter.WriteString(strings.Join(requests, "\n") + "\n")
	require.NoError(t, err)

	var lines []string

	scanner := bufio.NewScanner(stdoutReader)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())

		if strings.Contains(scanner.Text(), `"id":2`) {
			stdinWriter.Close()
		}
	}

	require.NoError(t, <-done)
	require.Len(t, lines, 2)

	for _, line := range lines {
		var message struct {
			JSONRPC string `json:"jsonrpc"`
		}

		require.NoError(t, json.Unmarshal([]byte(line), &message), line)
		assert.Equal(t, "2.0", message.JSONRPC)
	}

	assert.Contains(t, stderr.String(), "[+] fuku mcp · stdio · protocol ")
	assert.Contains(t, stderr.String(), " instance  not running · call run to start\n")
	assert.Regexp(t, `\n ✘ status {25}no fuku instance is running in .+; call run first · \d+ms\n$`, stderr.String())
}
