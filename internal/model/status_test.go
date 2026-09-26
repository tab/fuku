package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Status_IsRunning(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "running",
			status: StatusRunning,
			want:   true,
		},
		{
			name:   "stopped",
			status: StatusStopped,
			want:   false,
		},
		{
			name:   "starting",
			status: StatusStarting,
			want:   false,
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsRunning())
		})
	}
}
