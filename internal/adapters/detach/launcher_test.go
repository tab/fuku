package detach

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) {
	if os.Getenv("FUKU_TEST_DETACHED_CHILD") != "1" {
		os.Exit(m.Run())
	}

	sid, _ := unix.Getsid(0)
	fmt.Fprintf(os.Stderr, "%s leads=%t\n", strings.Join(os.Args[1:], " "), sid == os.Getpid())
}

func Test_Launcher_Launch(t *testing.T) {
	t.Setenv("FUKU_TEST_DETACHED_CHILD", "1")

	launcher := NewLauncher(Options{Profile: "core", ConfigFile: "fuku.ci.yaml"})

	child, err := launcher.Launch()
	require.NoError(t, err)

	t.Cleanup(func() { child.Wait() })

	line, err := bufio.NewReader(child.Output()).ReadString('\n')

	require.NoError(t, err)
	assert.Equal(t, "run core --no-ui --detached-child --config fuku.ci.yaml leads=true\n", line)
}

func Test_Launcher_args(t *testing.T) {
	tests := []struct {
		name     string
		options  Options
		expected []string
	}{
		{
			name:     "runs the profile headless as the child",
			options:  Options{Profile: "core"},
			expected: []string{"run", "core", "--no-ui", "--detached-child"},
		},
		{
			name:     "passes the config the parent loaded",
			options:  Options{Profile: "default", ConfigFile: "fuku.ci.yaml"},
			expected: []string{"run", "default", "--no-ui", "--detached-child", "--config", "fuku.ci.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NewLauncher(tt.options).args())
		})
	}
}

func Test_Child(t *testing.T) {
	tests := []struct {
		name   string
		script string
		end    func(child *Child) error
	}{
		{
			name:   "Terminate stops a running child, Wait reaps it",
			script: "echo started >&2; exec sleep 30",
			end: func(child *Child) error {
				require.NoError(t, child.Terminate())

				return child.Wait()
			},
		},
		{
			name:   "Release lets the child run on",
			script: "echo started >&2",
			end: func(child *Child) error {
				return child.Release()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			require.NoError(t, err)

			cmd := exec.Command("sh", "-c", tt.script)
			cmd.Stderr = writer

			require.NoError(t, cmd.Start())
			require.NoError(t, writer.Close())

			child := &Child{cmd: cmd, output: reader}

			line, err := bufio.NewReader(child.Output()).ReadString('\n')
			require.NoError(t, err)
			assert.Equal(t, "started\n", line)

			err = tt.end(child)

			if tt.name == "Release lets the child run on" {
				require.NoError(t, err)

				return
			}

			assert.EqualError(t, err, "signal: terminated")
		})
	}
}
