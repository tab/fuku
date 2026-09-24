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

	mockCleaner := NewMockCleaner(ctrl)

	ctx := t.Context()
	stopErr := errors.New("stop failed")

	tests := []struct {
		name         string
		before       func()
		profile      string
		expectedExit int
		expectedErr  error
	}{
		{
			name: "cleans up the profile",
			before: func() {
				mockCleaner.EXPECT().Cleanup(ctx, "core").Return(nil)
			},
			profile:      "core",
			expectedExit: 0,
		},
		{
			name: "cleanup failure exits 1",
			before: func() {
				mockCleaner.EXPECT().Cleanup(ctx, "failed").Return(stopErr)
			},
			profile:      "failed",
			expectedExit: 1,
			expectedErr:  stopErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exitCode, err := NewStop(&Options{Profile: tt.profile}, mockCleaner).Run(ctx)

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
