package logs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_NewRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

	r := NewRunner(mockClient)
	assert.NotNil(t, r)

	impl, ok := r.(*runner)
	assert.True(t, ok)
	assert.Equal(t, mockClient, impl.client)
}

func Test_parseArgs(t *testing.T) {
	r := &runner{}

	tests := []struct {
		name             string
		args             []string
		expectedProfile  string
		expectedServices []string
	}{
		{name: "Empty args", args: []string{}, expectedProfile: "", expectedServices: nil},
		{name: "Single service", args: []string{"api"}, expectedProfile: "", expectedServices: []string{"api"}},
		{name: "Multiple services", args: []string{"api", "db", "cache"}, expectedProfile: "", expectedServices: []string{"api", "db", "cache"}},
		{name: "Profile only", args: []string{"--profile=prod"}, expectedProfile: "prod", expectedServices: nil},
		{name: "Profile with services", args: []string{"--profile=dev", "api", "db"}, expectedProfile: "dev", expectedServices: []string{"api", "db"}},
		{name: "Skips unknown flags", args: []string{"--logs", "-v", "api"}, expectedProfile: "", expectedServices: []string{"api"}},
		{name: "Mixed args order", args: []string{"api", "--profile=staging", "db"}, expectedProfile: "staging", expectedServices: []string{"api", "db"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, services := r.parseArgs(tt.args)
			assert.Equal(t, tt.expectedProfile, profile)
			assert.Equal(t, tt.expectedServices, services)
		})
	}
}

func Test_streamLogs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)

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
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(errors.New("connection failed"))
			},
			expectedResult: 1,
		},
		{
			name:     "Subscribe error",
			services: []string{"api"},
			before: func() {
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{"api"}).Return(errors.New("subscribe failed"))
				mockClient.EXPECT().Close().Return(nil)
			},
			expectedResult: 1,
		},
		{
			name:     "Stream error",
			services: []string{"api"},
			before: func() {
				mockClient.EXPECT().Connect("/tmp/test.sock").Return(nil)
				mockClient.EXPECT().Subscribe([]string{"api"}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(errors.New("stream failed"))
				mockClient.EXPECT().Close().Return(nil)
			},
			expectedResult: 1,
		},
		{
			name:     "Writes to output",
			services: []string{},
			before: func() {
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

			r := NewRunner(mockClient).(*runner)

			var output bytes.Buffer

			result := r.streamLogs("/tmp/test.sock", tt.services, &output)
			assert.Equal(t, tt.expectedResult, result)

			if tt.expectedOutput != "" {
				assert.Equal(t, tt.expectedOutput, output.String())
			}
		})
	}
}
