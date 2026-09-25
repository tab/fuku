package logs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// serviceName is the subscription filter repeated across the session tests
const serviceName = "api"

func Test_Session_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockView := NewMockView(ctrl)

	identity := model.Instance{Project: "/Users/dev/projects/shop"}

	tail := 100
	connectErr := errors.New("connection refused")
	subscribeErr := errors.New("subscribe failed")
	streamErr := errors.New("stream interrupted")

	stream := func(_ context.Context, handler Handler) error {
		if err := handler.HandleStatus(contracts.LogStatus{Profile: "default", Version: "1.0.0", Services: []string{serviceName}}); err != nil {
			return err
		}

		handler.HandleLog(model.LogLine{Service: serviceName, Message: "hello from api"})

		return nil
	}

	tests := []struct {
		name    string
		request Request
		before  func()
		expect  error
	}{
		{
			name:    "success",
			request: Request{Services: []string{serviceName}},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName}, model.ReplayOptions{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(stream)
				mockView.EXPECT().Status(contracts.LogStatus{Profile: "default", Version: "1.0.0", Services: []string{serviceName}}, []string{serviceName})
				mockView.EXPECT().Line(model.LogLine{Service: serviceName, Message: "hello from api"})
				mockClient.EXPECT().Close().Return(nil)
			},
		},
		{
			name: "bounded read passes tail and no-follow",
			request: Request{
				Services:      []string{serviceName},
				ReplayOptions: model.ReplayOptions{Tail: &tail, NoFollow: true},
			},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName}, model.ReplayOptions{Tail: &tail, NoFollow: true}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(nil)
				mockClient.EXPECT().Close().Return(nil)
			},
		},
		{
			name: "matching profile is accepted",
			request: Request{
				Profile:  "default",
				Services: []string{serviceName},
			},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName}, model.ReplayOptions{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(stream)
				mockView.EXPECT().Status(gomock.Any(), []string{serviceName})
				mockView.EXPECT().Line(gomock.Any())
				mockClient.EXPECT().Close().Return(nil)
			},
		},
		{
			name: "mismatched profile ends the stream before the view sees it",
			request: Request{
				Profile:  "core",
				Services: []string{serviceName},
			},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName}, model.ReplayOptions{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(stream)
				mockClient.EXPECT().Close().Return(nil)
			},
			expect: contracts.ErrProfileMismatch,
		},
		{
			name: "no instance running",
			before: func() {
				mockClient.EXPECT().Connect().Return(contracts.ErrNoInstanceRunning)
			},
			expect: contracts.ErrNoInstanceRunning,
		},
		{
			name: "connect error",
			before: func() {
				mockClient.EXPECT().Connect().Return(connectErr)
			},
			expect: connectErr,
		},
		{
			name:    "subscribe error",
			request: Request{Services: []string{serviceName}},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName}, model.ReplayOptions{}).Return(subscribeErr)
				mockClient.EXPECT().Close().Return(nil)
			},
			expect: subscribeErr,
		},
		{
			name:    "stream error",
			request: Request{Services: []string{serviceName, "web"}},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe([]string{serviceName, "web"}, model.ReplayOptions{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).Return(streamErr)
				mockClient.EXPECT().Close().Return(nil)
			},
			expect: streamErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := NewSession(mockClient, mockView, identity).Run(t.Context(), tt.request)

			require.ErrorIs(t, err, tt.expect)
		})
	}
}

func Test_Session_Run_NamesTheContextOfTheFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockClient(ctrl)
	mockView := NewMockView(ctrl)

	identity := model.Instance{Project: "/Users/dev/projects/shop"}

	stream := func(_ context.Context, handler Handler) error {
		return handler.HandleStatus(contracts.LogStatus{Profile: "default"})
	}

	tests := []struct {
		name     string
		request  Request
		before   func()
		expected string
	}{
		{
			name: "a missing instance names the project",
			before: func() {
				mockClient.EXPECT().Connect().Return(contracts.ErrNoInstanceRunning)
			},
			expected: "no fuku instance is running for project '/Users/dev/projects/shop'",
		},
		{
			name:    "a profile mismatch names both profiles",
			request: Request{Profile: "core"},
			before: func() {
				mockClient.EXPECT().Connect().Return(nil)
				mockClient.EXPECT().Subscribe(nil, model.ReplayOptions{}).Return(nil)
				mockClient.EXPECT().Stream(gomock.Any(), gomock.Any()).DoAndReturn(stream)
				mockClient.EXPECT().Close().Return(nil)
			},
			expected: "fuku is running another profile: 'default' instead of 'core'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			err := NewSession(mockClient, mockView, identity).Run(t.Context(), tt.request)

			require.EqualError(t, err, tt.expected)
		})
	}
}
