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

	ctx := t.Context()
	stopErr := errors.New("stop failed")
	killErr := errors.New("operation not permitted")

	tests := []struct {
		name         string
		before       func()
		profile      string
		expectedExit int
		expectedErr  error
	}{
		{
			name: "stops the running instance, then cleans up the profile",
			before: func() {
				gomock.InOrder(
					mockInstance.EXPECT().Stop(ctx).Return(nil),
					mockCleaner.EXPECT().Cleanup(ctx, "core").Return(nil),
				)
			},
			profile:      "core",
			expectedExit: 0,
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

			exitCode, err := NewStop(&Options{Profile: tt.profile}, mockInstance, mockCleaner).Run(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
