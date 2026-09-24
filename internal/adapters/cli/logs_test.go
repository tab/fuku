package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/logs"
	"fuku/internal/contracts"
)

func Test_NewLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)

	request := logs.Request{Profile: "core"}

	c := NewLogs(request, mockSession)

	require.NotNil(t, c)
	assert.Equal(t, request, c.request)
	assert.Equal(t, mockSession, c.session)
}

func Test_Logs_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSession := NewMockSession(ctrl)

	request := logs.Request{Profile: "core", Services: []string{"api"}}
	streamErr := errors.New("stream interrupted")

	tests := []struct {
		name         string
		before       func()
		expectedExit int
		expectedErr  error
	}{
		{
			name: "streamed",
			before: func() {
				mockSession.EXPECT().Run(gomock.Any(), request).Return(nil)
			},
			expectedExit: 0,
		},
		{
			name: "profile mismatch",
			before: func() {
				mockSession.EXPECT().Run(gomock.Any(), request).Return(contracts.ErrProfileMismatch)
			},
			expectedExit: 1,
			expectedErr:  contracts.ErrProfileMismatch,
		},
		{
			name: "stream error",
			before: func() {
				mockSession.EXPECT().Run(gomock.Any(), request).Return(streamErr)
			},
			expectedExit: 1,
			expectedErr:  streamErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			exitCode, err := NewLogs(request, mockSession).Run(t.Context())

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}
