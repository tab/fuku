package cli

import (
	"context"
	"errors"
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
	runErr := errors.New("failed to resolve profile")

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name         string
		before       func() context.Context
		expectedExit int
		expectedErr  error
	}{
		{
			name: "clean run exits 0",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(finished)
				mockRuntime.EXPECT().Err().Return(nil)

				return t.Context()
			},
			expectedExit: 0,
		},
		{
			name: "failed run exits 1 with the runtime's error",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(finished)
				mockRuntime.EXPECT().Err().Return(runErr)

				return t.Context()
			},
			expectedExit: 1,
			expectedErr:  runErr,
		},
		{
			name: "cancelled run exits 0",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(finished)
				mockRuntime.EXPECT().Err().Return(context.Canceled)

				return t.Context()
			},
			expectedExit: 0,
		},
		{
			name: "an ended context exits 0 before the runtime finishes",
			before: func() context.Context {
				mockRuntime.EXPECT().Done().Return(pending)

				return cancelled
			},
			expectedExit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			exitCode, err := NewRun(mockRuntime).Run(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
