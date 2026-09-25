package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Run_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntime := NewMockRuntime(ctrl)

	finished := make(chan struct{})
	close(finished)

	pending := make(chan struct{})

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name   string
		before func() context.Context
	}{
		{
			name: "exits 0 once the runtime finishes",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(finished)

				return t.Context()
			},
		},
		{
			name: "exits 0 once the context ends before the runtime finishes",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(pending)

				return cancelled
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			exitCode, err := NewRun(mockRuntime).Run(ctx)

			require.NoError(t, err)
			assert.Equal(t, 0, exitCode)
		})
	}
}
