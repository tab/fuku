package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Server_redactCalls(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	type source struct{ project, path string }

	token := "secret-token"
	defaults := map[source]string{{dir, ""}: token}
	unchanged := fmt.Errorf("dial: %w", contracts.ErrNoInstanceRunning)
	call := func(arguments string) *sdk.CallToolRequest {
		return &sdk.CallToolRequest{Params: &sdk.CallToolParamsRaw{Name: toolLogs, Arguments: json.RawMessage(arguments)}}
	}
	typed := logsOutput{Lines: []logLine{{Service: "api", Message: "ready"}}}

	tests := []struct {
		name       string
		method     string
		request    sdk.Request
		root       string
		tokens     map[source]string
		loadErr    error
		result     sdk.Result
		err        error
		expected   string
		structured any
		failure    error
	}{
		{
			name:    "a log line with the token",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"lines":[{"message":"auth secret-token"}]}`}},
				StructuredContent: logsOutput{Lines: []logLine{{Service: "api", Message: "auth secret-token"}}},
			},
			expected:   `{"lines":[{"message":"auth [redacted]"}]}`,
			structured: map[string]any{"lines": []any{map[string]any{"service": "api", "message": "auth [redacted]"}}},
		},
		{
			name:     "a tool error with the token",
			method:   methodCallTool,
			request:  call(`{"config":"fuku.yaml"}`),
			tokens:   defaults,
			result:   &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "bad token secret-token"}}},
			expected: "bad token [redacted]",
		},
		{
			name:    "a protocol error with the token",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			err:     errors.New("dial secret-token"),
			failure: errors.New("dial [redacted]"),
		},
		{
			name:    "a protocol error without the token keeps its type",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			err:     unchanged,
			failure: unchanged,
		},
		{
			name:    "a JSON-RPC error with the token keeps its code",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			err:     &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "bad secret-token"},
			failure: &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "bad [redacted]"},
		},
		{
			name:    "a named config without a token redacts the token of the default config",
			method:  methodCallTool,
			request: call(`{"config":"missing.yaml"}`),
			tokens:  map[source]string{{dir, "missing.yaml"}: "", {dir, ""}: token},
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"lines":[{"message":"auth secret-token"}]}`}},
				StructuredContent: json.RawMessage(`{"lines":[{"message":"auth secret-token"}]}`),
			},
			expected:   `{"lines":[{"message":"auth [redacted]"}]}`,
			structured: map[string]any{"lines": []any{map[string]any{"message": "auth [redacted]"}}},
		},
		{
			name:    "a named config below the root redacts the token of its project",
			method:  methodCallTool,
			request: call(`{"config":"sub/fuku.yaml"}`),
			tokens:  map[source]string{{sub, "fuku.yaml"}: "sub-token", {dir, ""}: token},
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"lines":[{"message":"auth sub-token"}]}`}},
				StructuredContent: json.RawMessage(`{"lines":[{"message":"auth sub-token"}]}`),
			},
			expected:   `{"lines":[{"message":"auth [redacted]"}]}`,
			structured: map[string]any{"lines": []any{map[string]any{"message": "auth [redacted]"}}},
		},
		{
			name:    "a named config whose directory does not resolve redacts the token of the default config",
			method:  methodCallTool,
			request: call(`{"config":"missing/fuku.yaml"}`),
			tokens:  defaults,
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"lines":[{"message":"auth secret-token"}]}`}},
				StructuredContent: json.RawMessage(`{"lines":[{"message":"auth secret-token"}]}`),
			},
			expected:   `{"lines":[{"message":"auth [redacted]"}]}`,
			structured: map[string]any{"lines": []any{map[string]any{"message": "auth [redacted]"}}},
		},
		{
			name:    "a project root that does not resolve redacts nothing",
			method:  methodCallTool,
			request: call(`{}`),
			root:    "missing",
			tokens:  map[source]string{{"", ""}: token},
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: "auth secret-token"}},
				StructuredContent: typed,
			},
			expected:   "auth secret-token",
			structured: typed,
		},
		{
			name:    "a config that fails validation still redacts its token",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			loadErr: contracts.ErrInvalidConfig,
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"lines":[{"message":"auth secret-token"}]}`}},
				StructuredContent: json.RawMessage(`{"lines":[{"message":"auth secret-token"}]}`),
			},
			expected:   `{"lines":[{"message":"auth [redacted]"}]}`,
			structured: map[string]any{"lines": []any{map[string]any{"message": "auth [redacted]"}}},
		},
		{
			name:    "a token that JSON escapes",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  map[source]string{{dir, ""}: "s3cr&t-token"},
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"message":"auth s3cr\u0026t-token"}`}},
				StructuredContent: json.RawMessage(`{"message":"auth s3cr\u0026t-token"}`),
			},
			expected:   `{"message":"auth [redacted]"}`,
			structured: map[string]any{"message": "auth [redacted]"},
		},
		{
			name:    "a token that breaks the structured content drops it",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  map[source]string{{dir, ""}: "4242"},
			result: &sdk.CallToolResult{
				Content:           []sdk.Content{&sdk.TextContent{Text: `{"pid":4242}`}},
				StructuredContent: json.RawMessage(`{"pid":4242}`),
			},
			expected: `{"pid":[redacted]}`,
		},
		{
			name:       "a result without the token keeps its type",
			method:     methodCallTool,
			request:    call(`{}`),
			tokens:     defaults,
			result:     &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ready"}}, StructuredContent: typed},
			expected:   "ready",
			structured: typed,
		},
		{
			name:       "a project with no token",
			method:     methodCallTool,
			request:    call(`{}`),
			result:     &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "secret-token"}}, StructuredContent: typed},
			expected:   "secret-token",
			structured: typed,
		},
		{
			name:    "a nil result",
			method:  methodCallTool,
			request: call(`{}`),
			tokens:  defaults,
			result:  (*sdk.CallToolResult)(nil),
		},
		{
			name:    "another method",
			method:  "tools/list",
			request: &sdk.ListToolsRequest{Params: &sdk.ListToolsParams{}},
			tokens:  defaults,
			result:  &sdk.ListToolsResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envProjectDir, filepath.Join(dir, tt.root))

			subject := &Server{load: func(project, path string) model.Config {
				return model.Config{Project: model.Project{Server: model.Server{Token: tt.tokens[source{project, path}]}}, Error: tt.loadErr}
			}}
			next := func(context.Context, string, sdk.Request) (sdk.Result, error) { return tt.result, tt.err }

			result, err := subject.redactCalls(next)(t.Context(), tt.method, tt.request)

			assert.Equal(t, tt.failure, err)

			got, ok := result.(*sdk.CallToolResult)
			if !ok || got == nil {
				assert.Equal(t, tt.result, result)

				return
			}

			assert.Equal(t, tt.expected, got.Content[0].(*sdk.TextContent).Text)
			assert.Equal(t, tt.structured, got.StructuredContent)
		})
	}
}
