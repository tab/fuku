package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Plain_Banner(t *testing.T) {
	header := "[+] fuku mcp · stdio · protocol 2025-06-18"
	project := " project   /work/shop · 3f2a9c1d"
	tools := " tools     config doctor logs run service status stop"
	footer := "Serving MCP · close stdin or Ctrl+C to stop"

	tests := []struct {
		name     string
		state    InstanceState
		detail   string
		expected string
	}{
		{
			name:     "running",
			state:    InstanceStateRunning,
			detail:   "profile backend · http://127.0.0.1:9876 · pid 4242",
			expected: " instance  running · profile backend · http://127.0.0.1:9876 · pid 4242",
		},
		{
			name:     "not running",
			state:    InstanceStateNotRunning,
			detail:   "call run to start",
			expected: " instance  not running · call run to start",
		},
		{
			name:     "unknown",
			state:    InstanceStateUnknown,
			detail:   "failed to read the status: connection reset",
			expected: " instance  unknown · failed to read the status: connection reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr strings.Builder

			console := NewPlain(&stderr)
			console.Open()
			console.Banner(Banner{
				Protocol:    "2025-06-18",
				Project:     "/work/shop",
				Fingerprint: "3f2a9c1d",
				State:       tt.state,
				Detail:      tt.detail,
				Tools:       []string{"config", "doctor", "logs", "run", "service", "status", "stop"},
			})
			console.Close()

			assert.Equal(t, strings.Join([]string{header, project, tt.expected, tools, footer}, "\n")+"\n", stderr.String())
		})
	}
}

func Test_Plain_Call(t *testing.T) {
	tests := []struct {
		name     string
		call     Call
		expected string
	}{
		{
			name: "a successful call",
			call: Call{
				Target:   "doctor",
				Duration: 12 * time.Millisecond,
			},
			expected: " ✔ doctor                         12ms\n",
		},
		{
			name: "a service call with its settled state",
			call: Call{
				Target:   "service api restart",
				Outcome:  "running",
				Duration: 2140 * time.Millisecond,
			},
			expected: " ✔ service api restart  running   2.1s\n",
		},
		{
			name: "an outcome wider than its column",
			call: Call{
				Target:   "service api start",
				Outcome:  "starting · in progress",
				Duration: 45 * time.Second,
			},
			expected: " ✔ service api start    starting · in progress 45.0s\n",
		},
		{
			name: "a failed call",
			call: Call{
				Target:   "run",
				Failure:  "fuku run failed · exit 3",
				Duration: 7 * time.Millisecond,
			},
			expected: " ✘ run                            fuku run failed · exit 3 · 7ms\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr strings.Builder

			NewPlain(&stderr).Call(tt.call)

			assert.Equal(t, tt.expected, stderr.String())
		})
	}
}
