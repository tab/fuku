package detach

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_encode(t *testing.T) {
	line := encode(Record{Kind: KindReady, Service: "api", Duration: 2 * time.Second})

	assert.JSONEq(t, `{"kind":"ready","service":"api","duration":2000000000}`, string(line[:len(line)-1]))
	assert.Equal(t, byte('\n'), line[len(line)-1])
}

func Test_decode(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected Record
		ok       bool
	}{
		{
			name:     "a record",
			line:     `{"kind":"running","pid":42,"count":3}`,
			expected: Record{Kind: KindRunning, PID: 42, Count: 3},
			ok:       true,
		},
		{
			name: "an error the child printed",
			line: "Error: fuku is already running for this project",
		},
		{
			name: "a line that looks like JSON but is not",
			line: "{not json",
		},
		{
			name: "a JSON line without a kind",
			line: `{"service":"api"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, ok := decode([]byte(tt.line))

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, record)
		})
	}
}
