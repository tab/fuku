package mcp

import (
	"context"
	"io"
	"os"
	"path/filepath"
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

func Test_Server_withConfig(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	sub := filepath.Join(root, "sub")
	args := []string{"run", "default"}

	tests := []struct {
		name        string
		defaultFile string
		config      string
		dir         string
		expected    []string
	}{
		{
			name:     "the default lookup",
			dir:      root,
			expected: args,
		},
		{
			name:        "the default file of the project",
			defaultFile: filepath.Join(root, "fuku.yaml"),
			config:      "fuku.yaml",
			dir:         root,
			expected:    args,
		},
		{
			name:        "the default file from the current directory",
			defaultFile: filepath.Join(root, "fuku.yml"),
			config:      "./fuku.yml",
			dir:         root,
			expected:    args,
		},
		{
			name:        "an absolute path to the default file",
			defaultFile: filepath.Join(root, "fuku.yaml"),
			config:      filepath.Join(root, "fuku.yaml"),
			dir:         root,
			expected:    args,
		},
		{
			name:        "the default file of a project below the root",
			defaultFile: filepath.Join(sub, "fuku.yaml"),
			config:      "sub/fuku.yaml",
			dir:         sub,
			expected:    args,
		},
		{
			name:        "a fuku.yml beside the default fuku.yaml",
			defaultFile: filepath.Join(root, "fuku.yaml"),
			config:      "fuku.yml",
			dir:         root,
			expected:    append(args, "--config", filepath.Join(root, "fuku.yml")),
		},
		{
			name:     "a fuku.yaml without a default file",
			config:   "fuku.yaml",
			dir:      root,
			expected: append(args, "--config", filepath.Join(root, "fuku.yaml")),
		},
		{
			name:        "another file in the project",
			defaultFile: filepath.Join(sub, "fuku.yaml"),
			config:      "sub/custom.yaml",
			dir:         sub,
			expected:    append(args, "--config", filepath.Join(sub, "custom.yaml")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := &Server{load: func(dir, path string) model.Config {
				assert.Equal(t, tt.dir, dir)
				assert.Empty(t, path)

				return model.Config{Path: tt.defaultFile}
			}}

			result := subject.withConfig(args, tt.config, tt.dir)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_projectDir(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	wd, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	other, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(wd, "sub"), 0o755))
	t.Chdir(wd)

	tests := []struct {
		name          string
		env           string
		config        string
		expected      string
		expectedError error
	}{
		{
			name:     "the working directory without the variable",
			expected: wd,
		},
		{
			name:     "the variable over the working directory",
			env:      root,
			expected: root,
		},
		{
			name:     "the variable through a symlink",
			env:      link,
			expected: root,
		},
		{
			name:     "a relative config under the variable",
			env:      root,
			config:   "sub/fuku.yaml",
			expected: filepath.Join(root, "sub"),
		},
		{
			name:     "a relative config in the working directory",
			config:   "sub/fuku.yaml",
			expected: filepath.Join(wd, "sub"),
		},
		{
			name:     "a bare config file name under the variable",
			env:      root,
			config:   "custom.yaml",
			expected: root,
		},
		{
			name:     "an absolute config over the variable",
			env:      root,
			config:   filepath.Join(other, "fuku.yaml"),
			expected: other,
		},
		{
			name:          "a missing config directory",
			env:           root,
			config:        "missing/fuku.yaml",
			expectedError: instance.ErrFailedToResolveProject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envProjectDir, tt.env)

			result, err := projectDir(tt.config)

			require.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Server_ProjectOverWorkingDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockExecutor := NewMockExecutor(ctrl)

	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	wd, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	t.Setenv(envProjectDir, project)
	t.Chdir(wd)

	configs := map[string]model.Config{
		project: {Project: model.Project{
			Services: []model.Service{{Name: "api", Command: "make run TOKEN=project-token", Directory: filepath.Join(project, "api"), Tier: model.TierDefault}},
			Profiles: map[string]model.Profile{"backend": {All: true}},
			Server:   model.Server{Token: "project-token"},
		}},
		wd: {Project: model.Project{
			Services: []model.Service{{Name: "web", Command: "make run TOKEN=wd-token", Directory: filepath.Join(wd, "web"), Tier: model.TierDefault}},
			Profiles: map[string]model.Profile{"frontend": {All: true}},
			Server:   model.Server{Token: "wd-token"},
		}},
	}

	newInstance := func(identity model.Instance) Instance {
		assert.Equal(t, project, identity.Project)

		return mockInstance
	}

	var newClient func(string, string) (Client, error)

	load := func(dir, _ string) model.Config { return configs[dir] }
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
		name     string
		before   func()
		tool     string
		expected string
	}{
		{
			name:     "config reads the project and redacts its token",
			before:   func() {},
			tool:     toolConfig,
			expected: `{"api":{"enabled":false,"listen":""},"profiles":{"backend":["api"]},"services":[{"command":"make run TOKEN=[redacted]","dir":"api","name":"api","tier":"default"}]}`,
		},
		{
			name: "status names the profiles of the project",
			before: func() {
				mockInstance.EXPECT().Status().Return(contracts.LogStatus{}, contracts.ErrNoInstanceRunning)
			},
			tool:     toolStatus,
			expected: `{"mode":"","note":"no fuku runs in ` + project + `; call run with one of: backend","phase":"","profile":"","profiles":["backend"],"running":false,"services":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()

			result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tt.tool, Arguments: map[string]any{}})

			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.False(t, result.IsError)
			assert.Equal(t, tt.expected, result.Content[0].(*sdk.TextContent).Text)
		})
	}
}
