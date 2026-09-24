package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Snapshot_Counts(t *testing.T) {
	tests := []struct {
		name     string
		snapshot Snapshot
		expected Counts
	}{
		{
			name:     "empty",
			snapshot: Snapshot{},
			expected: Counts{},
		},
		{
			name: "one service per status",
			snapshot: Snapshot{Services: map[string]*Service{
				"id-pending":    {Status: StatusPending},
				"id-starting":   {Status: StatusStarting},
				"id-running":    {Status: StatusRunning},
				"id-stopping":   {Status: StatusStopping},
				"id-restarting": {Status: StatusRestarting},
				"id-stopped":    {Status: StatusStopped},
				"id-failed":     {Status: StatusFailed},
			}},
			expected: Counts{Total: 7, Pending: 1, Starting: 1, Running: 1, Stopping: 1, Restarting: 1, Stopped: 1, Failed: 1},
		},
		{
			name: "several services share a status",
			snapshot: Snapshot{Services: map[string]*Service{
				"id-api": {Status: StatusRunning},
				"id-web": {Status: StatusRunning},
				"id-db":  {Status: StatusFailed},
			}},
			expected: Counts{Total: 3, Running: 2, Failed: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.snapshot.Counts())
		})
	}
}
