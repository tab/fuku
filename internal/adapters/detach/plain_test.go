package detach

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Plain_Show(t *testing.T) {
	tests := []struct {
		name     string
		record   Record
		expected string
	}{
		{
			name:     "a starting service",
			record:   Record{Kind: KindStarting, Service: "api"},
			expected: " • api Starting\n",
		},
		{
			name:     "a ready service with its startup time",
			record:   Record{Kind: KindReady, Service: "api", Duration: 1250 * time.Millisecond},
			expected: " ✔ api Ready 1.2s\n",
		},
		{
			name:     "a failed service with its error",
			record:   Record{Kind: KindFailed, Service: "api", Error: "max retries exceeded"},
			expected: " ✗ api Failed: max retries exceeded\n",
		},
		{
			name:   "a record without a line",
			record: Record{Kind: KindRunning, PID: 42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout strings.Builder

			view := NewPlain(&stdout)
			view.Open()
			view.Show(tt.record)
			view.Close()

			assert.Equal(t, tt.expected, stdout.String())
		})
	}
}
