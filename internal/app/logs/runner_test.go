package logs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config/logger"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)

	r := NewRunner(mockClient, mockLogger)
	assert.NotNil(t, r)

	impl, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, mockClient, impl.client)
	assert.Equal(t, componentLogger, impl.log)
}

func Test_streamLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name           string
		services       []string
		before         func()
		expectedResult int
		expectedOutput string
	}{
		{
			name:     "Success",
			services: []string{"api"},
			before: func() {
				mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{"api"}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
				mockClient.EXPECT().Close().Return(nil)
			},
			expectedResult: 0,
		},
		{
			name:     "Connect error",
			services: []string{"api"},
			before: func() {
				mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(errors.New("connection failed"))
				componentLogger.EXPECT().Error().Return(nil)
			},
			expectedResult: 1,
		},
		{
			name:     "Subscribe error",
			services: []string{"api"},
			before: func() {
				mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{"api"}).Return(errors.New("subscribe failed"))
				mockClient.EXPECT().Close().Return(nil)
				componentLogger.EXPECT().Error().Return(nil)
			},
			expectedResult: 1,
		},
		{
			name:     "Stream error",
			services: []string{"api"},
			before: func() {
				mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{"api"}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(errors.New("stream failed"))
				mockClient.EXPECT().Close().Return(nil)
				componentLogger.EXPECT().Error().Return(nil)
			},
			expectedResult: 1,
		},
		{
			name:     "Writes to output",
			services: []string{},
			before: func() {
				mockLogger.EXPECT().WithComponent("LOGS").Return(componentLogger)
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, w io.Writer) error {
					w.Write([]byte("log output"))
					return nil
				})
				mockClient.EXPECT().Close().Return(nil)
			},
			expectedResult: 0,
			expectedOutput: "log output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			r := NewRunner(mockClient, mockLogger).(*runner)

			var output bytes.Buffer

			result := r.streamLogs("/tmp/test.sock", tt.services, &output)
			assert.Equal(t, tt.expectedResult, result)

			if tt.expectedOutput != "" {
				assert.Equal(t, tt.expectedOutput, output.String())
			}
		})
	}
}
