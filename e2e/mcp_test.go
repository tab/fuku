package e2e

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mcpTools lists the tools in the order tools/list returns them
var mcpTools = []string{"config", "doctor", "logs", "run", "service", "status", "stop"}

// mcpCallLine matches the one stderr line the server writes per tool call
var mcpCallLine = regexp.MustCompile(`(?m)^ [✔✘] .*$`)

// mcpClient drives `fuku mcp` over its stdin and stdout, one JSON-RPC message per line
type mcpClient struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	read   []string
	stderr *lockedBuffer
	nextID int
}

// mcpReply is a JSON-RPC response of the server
type mcpReply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// mcpResult is the result of one tool call: its text, its structured content and whether it is a tool error
type mcpResult struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

// startMCP starts `fuku mcp` with dir as the project and completes the handshake, keeping stdin open
func startMCP(t *testing.T, dir string) *mcpClient {
	t.Helper()

	bin := os.Getenv("FUKU_BIN")
	if bin == "" {
		bin = "fuku"
	}

	workDir, err := filepath.Abs(dir)
	require.NoError(t, err)

	cmd := exec.Command(bin, "mcp")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+workDir)

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	client := &mcpClient{t: t, cmd: cmd, stdin: stdin, lines: make(chan string, 64), stderr: &lockedBuffer{}}
	cmd.Stderr = client.stderr

	require.NoError(t, cmd.Start())
	t.Cleanup(func() { cmd.Process.Kill() })

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

		for scanner.Scan() {
			client.lines <- scanner.Text()
		}

		close(client.lines)
	}()

	client.request("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "e2e", "version": "0"},
	}, 10*time.Second)
	client.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})

	return client
}

// send writes one JSON-RPC message to the server's stdin
func (c *mcpClient) send(message map[string]any) {
	c.t.Helper()

	data, err := json.Marshal(message)
	require.NoError(c.t, err)

	_, err = c.stdin.Write(append(data, '\n'))
	require.NoError(c.t, err)
}

// request sends a request and returns the result of its reply, failing on a JSON-RPC error or a timeout
func (c *mcpClient) request(method string, params any, timeout time.Duration) json.RawMessage {
	c.t.Helper()

	c.nextID++
	id := c.nextID
	c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})

	deadline := time.After(timeout)

	for {
		select {
		case line, ok := <-c.lines:
			require.True(c.t, ok, "stdout closed before the reply to %s\nStderr:\n%s", method, c.stderr.String())
			c.read = append(c.read, line)

			var reply mcpReply
			if json.Unmarshal([]byte(line), &reply) != nil || reply.ID == nil || *reply.ID != id {
				continue
			}

			require.Empty(c.t, reply.Error, "%s failed", method)

			return reply.Result
		case <-deadline:
			require.FailNow(c.t, "timeout waiting for the reply to "+method, "Stderr:\n%s", c.stderr.String())
		}
	}
}

// tools returns the tool names tools/list answers, in its order
func (c *mcpClient) tools() []string {
	c.t.Helper()

	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}

	require.NoError(c.t, json.Unmarshal(c.request("tools/list", map[string]any{}, 10*time.Second), &listed))

	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}

	return names
}

// call calls a tool and returns its result
func (c *mcpClient) call(tool string, arguments map[string]any, timeout time.Duration) mcpResult {
	c.t.Helper()

	var result mcpResult

	raw := c.request("tools/call", map[string]any{"name": tool, "arguments": arguments}, timeout)
	require.NoError(c.t, json.Unmarshal(raw, &result))

	return result
}

// close closes stdin, waits for the server to exit, reads what stdout still held and returns the exit code
func (c *mcpClient) close() int {
	c.t.Helper()

	require.NoError(c.t, c.stdin.Close())

	for line := range c.lines {
		c.read = append(c.read, line)
	}

	done := make(chan error, 1)

	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case err := <-done:
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode()
		}

		require.NoError(c.t, err)

		return 0
	case <-time.After(10 * time.Second):
		c.cmd.Process.Kill()
		<-done
		require.FailNow(c.t, "fuku mcp did not exit after stdin closed")

		return -1
	}
}

// assertProtocolOnly asserts that every stdout line is a JSON-RPC message
func (c *mcpClient) assertProtocolOnly() {
	c.t.Helper()

	require.NotEmpty(c.t, c.read)

	for _, line := range c.read {
		var reply mcpReply
		if assert.NoError(c.t, json.Unmarshal([]byte(line), &reply), "stdout line is not JSON: %q", line) {
			assert.Equal(c.t, "2.0", reply.JSONRPC, "stdout line is not JSON-RPC: %q", line)
		}
	}
}

// structured decodes the structured content of a successful tool result
func structured(t *testing.T, result mcpResult, out any) {
	t.Helper()

	require.False(t, result.IsError, "tool error: %+v", result.Content)
	require.NoError(t, json.Unmarshal(result.StructuredContent, out))
}

func Test_MCP_ServesWithoutConfig(t *testing.T) {
	client := startMCP(t, t.TempDir())

	assert.Equal(t, mcpTools, client.tools())

	var status struct {
		Running  bool              `json:"running"`
		Services []json.RawMessage `json:"services"`
		Note     string            `json:"note"`
	}

	structured(t, client.call("status", map[string]any{}, 10*time.Second), &status)

	assert.False(t, status.Running)
	assert.NotNil(t, status.Services)
	assert.Empty(t, status.Services)
	assert.Contains(t, status.Note, "no fuku runs in")

	logs := client.call("logs", map[string]any{}, 10*time.Second)

	assert.True(t, logs.IsError)
	require.NotEmpty(t, logs.Content)
	assert.Contains(t, logs.Content[0].Text, "no fuku instance is running")
	assert.Equal(t, mcpTools, client.tools(), "the server must keep serving after a tool error")

	assert.Equal(t, 0, client.close())
	client.assertProtocolOnly()

	stderr := client.stderr.String()
	assert.Contains(t, stderr, "fuku mcp · stdio · protocol")
	assert.Contains(t, stderr, "not running")
	assert.Contains(t, stderr, "config doctor logs run service status stop")

	calls := mcpCallLine.FindAllString(stderr, -1)
	require.Len(t, calls, 2, "one line per tool call in:\n%s", stderr)
	assert.Regexp(t, `^ ✔ status `, calls[0])
	assert.Regexp(t, `^ ✘ logs .*no fuku instance is running.* · \d+ms$`, calls[1])
}

func Test_MCP_ControlsARunningProfile(t *testing.T) {
	client := startMCP(t, detachedDir)

	var run struct {
		ExitCode int    `json:"exitCode"`
		Summary  string `json:"summary"`
	}

	structured(t, client.call("run", map[string]any{"profile": "default"}, 60*time.Second), &run)

	pid := detachedPID(t, run.Summary)
	defer endDetached(pid)

	assert.Equal(t, 0, run.ExitCode)
	assert.Contains(t, run.Summary, "http://127.0.0.1:19890")

	type serviceState struct {
		Name  string `json:"name"`
		State string `json:"state"`
		PID   int    `json:"pid"`
	}

	var status struct {
		Profile  string         `json:"profile"`
		Phase    string         `json:"phase"`
		Mode     string         `json:"mode"`
		Services []serviceState `json:"services"`
	}

	structured(t, client.call("status", map[string]any{}, 10*time.Second), &status)

	assert.Equal(t, "default", status.Profile)
	assert.Equal(t, "running", status.Phase)
	assert.Equal(t, "detached", status.Mode)

	states := map[string]serviceState{}
	for _, service := range status.Services {
		states[service.Name] = service
	}

	require.Contains(t, states, "worker")
	require.Contains(t, states, "api")
	assert.Equal(t, "running", states["api"].State)
	assert.Equal(t, "running", states["worker"].State)
	require.NotZero(t, states["worker"].PID)

	var restarted serviceState

	structured(t, client.call("service", map[string]any{"name": "worker", "action": "restart"}, 60*time.Second), &restarted)

	assert.Equal(t, "worker", restarted.Name)
	assert.Equal(t, "running", restarted.State)
	assert.NotZero(t, restarted.PID)
	assert.NotEqual(t, states["worker"].PID, restarted.PID, "a restart must start a new process")

	var logs struct {
		Lines []struct {
			Service string `json:"service"`
			Message string `json:"message"`
		} `json:"lines"`
	}

	structured(t, client.call("logs", map[string]any{"tail": 5}, 20*time.Second), &logs)

	assert.NotEmpty(t, logs.Lines)
	assert.LessOrEqual(t, len(logs.Lines), 5)

	var stop struct {
		ExitCode int    `json:"exitCode"`
		Summary  string `json:"summary"`
	}

	structured(t, client.call("stop", map[string]any{}, 30*time.Second), &stop)

	assert.Equal(t, 0, stop.ExitCode)
	assert.Equal(t, "stopped profile default", stop.Summary)
	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))
	assert.NoError(t, WaitForNoProcess("sleep 601", 10*time.Second))

	assert.Equal(t, 0, client.close())
	client.assertProtocolOnly()

	stderr := client.stderr.String()
	calls := mcpCallLine.FindAllString(stderr, -1)
	require.Len(t, calls, 5, "one line per tool call in:\n%s", stderr)
	assert.Regexp(t, `^ ✔ run `, calls[0])
	assert.Regexp(t, `^ ✔ status `, calls[1])
	assert.Regexp(t, `^ ✔ service worker restart +running `, calls[2])
	assert.Regexp(t, `^ ✔ logs `, calls[3])
	assert.Regexp(t, `^ ✔ stop `, calls[4])
}

func Test_MCP_RejectsWhatTheProfileCannotDo(t *testing.T) {
	client := startMCP(t, detachedDir)

	var config struct {
		Profiles map[string][]string `json:"profiles"`
		API      struct {
			Enabled bool   `json:"enabled"`
			Listen  string `json:"listen"`
		} `json:"api"`
	}

	structured(t, client.call("config", map[string]any{}, 10*time.Second), &config)

	assert.Equal(t, []string{"api", "worker"}, config.Profiles["default"])
	assert.True(t, config.API.Enabled)
	assert.Equal(t, "127.0.0.1:19890", config.API.Listen)

	var run struct {
		Summary string `json:"summary"`
	}

	structured(t, client.call("run", map[string]any{"profile": "default"}, 60*time.Second), &run)

	pid := detachedPID(t, run.Summary)
	defer endDetached(pid)

	again := client.call("run", map[string]any{"profile": "default"}, 60*time.Second)

	assert.True(t, again.IsError)
	require.NotEmpty(t, again.Content)
	assert.Contains(t, again.Content[0].Text, "already running")

	unknown := client.call("service", map[string]any{"name": "nope", "action": "restart"}, 20*time.Second)

	assert.True(t, unknown.IsError)
	require.NotEmpty(t, unknown.Content)
	assert.Contains(t, unknown.Content[0].Text, "unknown service nope")
	assert.Contains(t, unknown.Content[0].Text, "api")
	assert.Contains(t, unknown.Content[0].Text, "worker")

	stopped := client.call("service", map[string]any{"name": "worker", "action": "stop"}, 60*time.Second)

	assert.False(t, stopped.IsError)

	rejected := client.call("service", map[string]any{"name": "worker", "action": "stop"}, 60*time.Second)

	assert.True(t, rejected.IsError)
	require.NotEmpty(t, rejected.Content)
	assert.Contains(t, rejected.Content[0].Text, "fuku rejected the action")

	var stop struct {
		Summary string `json:"summary"`
	}

	structured(t, client.call("stop", map[string]any{}, 30*time.Second), &stop)

	assert.Equal(t, "stopped profile default", stop.Summary)
	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))

	assert.Equal(t, 0, client.close())
	client.assertProtocolOnly()

	secret := "test-token"
	assert.NotContains(t, strings.Join(client.read, "\n"), secret, "the agent must never see the API token")
	assert.NotContains(t, client.stderr.String(), secret, "the call lines must never show the API token")
}

func Test_MCP_ServiceNeedsTheAPI(t *testing.T) {
	t.Setenv("FUKU_API_DISABLED", "1")

	client := startMCP(t, "testdata/env")

	var run struct {
		Summary string `json:"summary"`
	}

	structured(t, client.call("run", map[string]any{"profile": "default"}, 60*time.Second), &run)

	pid := detachedPID(t, run.Summary)
	defer endDetached(pid)

	restart := client.call("service", map[string]any{"name": "printer", "action": "restart"}, 20*time.Second)

	assert.True(t, restart.IsError)
	require.NotEmpty(t, restart.Content)
	assert.Contains(t, restart.Content[0].Text, "REST API is off")
	assert.Contains(t, restart.Content[0].Text, "stop and run")

	var stop struct {
		Summary string `json:"summary"`
	}

	structured(t, client.call("stop", map[string]any{}, 30*time.Second), &stop)

	assert.Equal(t, "stopped profile default", stop.Summary)
	require.NoError(t, WaitForGroupExit(pid, 10*time.Second))

	assert.Equal(t, 0, client.close())
}

func Test_MCP_ReportsTheForegroundMode(t *testing.T) {
	runner := NewRunner(t, detachedDir)
	defer runner.Stop()

	require.NoError(t, runner.Start("default"))
	require.NoError(t, runner.WaitForRunning(30*time.Second))

	client := startMCP(t, detachedDir)

	var status struct {
		Mode string `json:"mode"`
	}

	structured(t, client.call("status", map[string]any{}, 10*time.Second), &status)

	assert.Equal(t, "foreground", status.Mode)
	assert.Equal(t, 0, client.close())
}
