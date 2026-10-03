package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Stop_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockInstance := NewMockInstance(ctrl)
	mockCleaner := NewMockCleaner(ctrl)
	mockSocket := NewMockSocket(ctrl)

	ctx := t.Context()
	stopErr := errors.New("stop failed")
	killErr := errors.New("operation not permitted")
	removeErr := errors.New("permission denied")

	tests := []struct {
		name         string
		before       func()
		profile      string
		expectedExit int
		expectedErr  error
	}{
		{
			name: "stops the running instance, cleans up the profile, then removes the stale socket",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Stop(ctx).Return(nil),
					mockCleaner.EXPECT().Cleanup(ctx, "core").Return(nil),
					mockSocket.EXPECT().Remove().Return(nil),
				)
			},
			profile:      "core",
			expectedExit: 0,
		},
		{
			name: "a socket that cannot be removed exits 1",
			before: func() {
				mockInstance.EXPECT().Stop(ctx).Return(nil)
				mockCleaner.EXPECT().Cleanup(ctx, "core").Return(nil)
				mockSocket.EXPECT().Remove().Return(removeErr)
			},
			profile:      "core",
			expectedExit: 1,
			expectedErr:  removeErr,
		},
		{
			name: "cleanup failure exits 1",
			before: func() {
				mockInstance.EXPECT().Stop(ctx).Return(nil)
				mockCleaner.EXPECT().Cleanup(ctx, "failed").Return(stopErr)
			},
			profile:      "failed",
			expectedExit: 1,
			expectedErr:  stopErr,
		},
		{
			name: "an instance that cannot be stopped exits 1 before the cleanup",
			before: func() {
				mockInstance.EXPECT().Stop(ctx).Return(killErr)
			},
			profile:      "core",
			expectedExit: 1,
			expectedErr:  killErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exitCode, err := NewStop(&Options{Profile: tt.profile}, mockInstance, mockCleaner, mockSocket).Run(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
