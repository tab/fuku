package updater

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Checker_run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSource := NewMockReleaseSource(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)

	tests := []struct {
		name    string
		before  func()
		options Options
	}{
		{
			name:    "disabled asks nothing and publishes nothing",
			before:  func() {},
			options: Options{Enabled: false, Version: "0.19.1"},
		},
		{
			name: "newer release is published normalized",
			before: func() {
				mockSource.EXPECT().Latest(gomock.Any()).Return(model.Release{Tag: "0.20.0"}, nil)
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventUpdateAvailable,
					Data: contracts.UpdateAvailable{Version: "v0.20.0"},
				}).Return(nil)
			},
			options: Options{Enabled: true, Version: "0.19.1"},
		},
		{
			name: "same version publishes nothing",
			before: func() {
				mockSource.EXPECT().Latest(gomock.Any()).Return(model.Release{Tag: "v0.19.1"}, nil)
			},
			options: Options{Enabled: true, Version: "0.19.1"},
		},
		{
			name: "older release publishes nothing",
			before: func() {
				mockSource.EXPECT().Latest(gomock.Any()).Return(model.Release{Tag: "v0.10.0"}, nil)
			},
			options: Options{Enabled: true, Version: "0.19.1"},
		},
		{
			name: "invalid release tag publishes nothing",
			before: func() {
				mockSource.EXPECT().Latest(gomock.Any()).Return(model.Release{Tag: "not-a-version"}, nil)
			},
			options: Options{Enabled: true, Version: "0.19.1"},
		},
		{
			name: "source failure is non-fatal",
			before: func() {
				mockSource.EXPECT().Latest(gomock.Any()).Return(model.Release{}, assert.AnError)
			},
			options: Options{Enabled: true, Version: "0.19.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			NewChecker(tt.options, mockSource, mockPublisher, log).run(t.Context())
		})
	}
}

func Test_Checker_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSource := NewMockReleaseSource(ctrl)
	mockPublisher := NewMockPublisher(ctrl)

	log := slog.New(slog.DiscardHandler)
	options := Options{Enabled: true, Version: "0.19.1"}

	tests := []struct {
		name     string
		before   func() (*Checker, context.Context)
		expected error
	}{
		{
			name: "stop abandons a check that is still fetching",
			before: func() (*Checker, context.Context) {
				checker := NewChecker(options, mockSource, mockPublisher, log)
				fetching := make(chan struct{})
				waitForCancel := func(ctx context.Context) (model.Release, error) {
					close(fetching)
					<-ctx.Done()

					return model.Release{}, ctx.Err()
				}

				mockSource.EXPECT().Latest(gomock.Any()).DoAndReturn(waitForCancel)

				require.NoError(t, checker.Start(t.Context()))

				<-fetching

				return checker, t.Context()
			},
		},
		{
			name: "stop gives up once its context ends on a check that ignores the cancel",
			before: func() (*Checker, context.Context) {
				checker := NewChecker(options, mockSource, mockPublisher, log)
				fetching := make(chan struct{})
				release := make(chan struct{})
				ignoreCancel := func(context.Context) (model.Release, error) {
					close(fetching)
					<-release

					return model.Release{}, context.Canceled
				}

				t.Cleanup(func() { close(release) })

				mockSource.EXPECT().Latest(gomock.Any()).DoAndReturn(ignoreCancel)

				require.NoError(t, checker.Start(t.Context()))

				<-fetching

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return checker, ctx
			},
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, ctx := tt.before()

			err := checker.Stop(ctx)

			require.ErrorIs(t, err, tt.expected)
		})
	}
}
