package resources

import (
	"context"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// readFrom stands in for Registry.Read and runs the callback on the fixture snapshot
func readFrom(snapshot *model.Snapshot) func(func(*model.Snapshot)) {
	return func(fn func(*model.Snapshot)) {
		fn(snapshot)
	}
}

func Test_NewSampler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	options := Options{Enabled: true}

	s := NewSampler(options, mockPublisher, mockMonitor, mockRegistry)

	assert.NotNil(t, s)
	assert.Equal(t, options, s.options)
	assert.Equal(t, mockPublisher, s.publisher)
	assert.Equal(t, mockMonitor, s.monitor)
	assert.Equal(t, mockRegistry, s.registry)
}

func Test_Sampler_run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	enabled := NewSampler(Options{Enabled: true}, mockPublisher, mockMonitor, mockRegistry)
	disabled := NewSampler(Options{}, mockPublisher, mockMonitor, mockRegistry)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name    string
		before  func()
		subject *Sampler
	}{
		{
			name: "primes the process usage when enabled and returns once cancelled",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(Stats{}, nil)
			},
			subject: enabled,
		},
		{
			name:    "reads nothing before the first tick when disabled",
			before:  func() {},
			subject: disabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			tt.subject.run(ctx)
		})
	}
}

func Test_Sampler_run_SamplesOnEachTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPublisher := NewMockPublisher(ctrl)
		mockMonitor := NewMockMonitor(ctrl)
		mockRegistry := NewMockRegistry(ctrl)

		sampler := NewSampler(Options{Enabled: true}, mockPublisher, mockMonitor, mockRegistry)

		idle := &model.Snapshot{}

		mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).Return(Stats{}, nil).Times(2)
		mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(idle)).Times(int(processInterval / serviceInterval))

		require.NoError(t, sampler.Start(t.Context()))

		<-time.After(processInterval + serviceInterval/2)

		require.NoError(t, sampler.Stop(t.Context()))
	})
}

func Test_Sampler_sampleProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	s := NewSampler(Options{Enabled: true}, mockPublisher, mockMonitor, mockRegistry)

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "publishes the usage once the CPU delta is warm",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(Stats{CPU: 2.5, MEM: 64.0, RawMEM: 64 * 1024 * 1024}, nil)
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventResourceSampled,
					Data: contracts.ResourceSampled{CPU: 2.5, Memory: 64 * 1024 * 1024},
				}).Return(nil)
			},
		},
		{
			name: "skips a sample with no usage",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(Stats{}, nil)
			},
		},
		{
			name: "skips a failed read",
			before: func() {
				mockMonitor.EXPECT().GetStats(gomock.Any(), gomock.Any()).Return(Stats{}, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.sampleProcess(t.Context())
		})
	}
}

func Test_Sampler_sampleServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	s := NewSampler(Options{}, mockPublisher, mockMonitor, mockRegistry)

	tests := []struct {
		name   string
		before func()
	}{
		{
			name: "publishes one batch of every live service, skipping a failed read",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Tiers: []*model.Tier{
					{Name: "platform", Services: []*model.Service{{ID: "test-id-api", Process: model.Process{PID: 1234}}, {ID: "test-id-web", Process: model.Process{PID: 0}}}},
					{Name: "foundation", Services: []*model.Service{{ID: "test-id-db", Process: model.Process{PID: 5678}}, {ID: "test-id-cache", Process: model.Process{PID: 9012}}}},
				}}))
				mockMonitor.EXPECT().GetStats(gomock.Any(), 1234).Return(Stats{CPU: 2.5, RawMEM: 4096}, nil)
				mockMonitor.EXPECT().GetStats(gomock.Any(), 5678).Return(Stats{}, assert.AnError)
				mockMonitor.EXPECT().GetStats(gomock.Any(), 9012).Return(Stats{CPU: 0, RawMEM: 2048}, nil)
				mockPublisher.EXPECT().Publish(contracts.Message{
					Type: contracts.EventServiceResourcesSampled,
					Data: contracts.ServiceResourcesSampled{Services: []contracts.ServiceResourceSample{
						{ID: "test-id-api", PID: 1234, CPU: 2.5, Memory: 4096},
						{ID: "test-id-cache", PID: 9012, CPU: 0, Memory: 2048},
					}},
				}).Return(nil)
			},
		},
		{
			name: "publishes nothing without a live service",
			before: func() {
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(&model.Snapshot{Tiers: []*model.Tier{
					{Name: "platform", Services: []*model.Service{{ID: "test-id-api"}}},
				}}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			s.sampleServices(t.Context())
		})
	}
}

func Test_Sampler_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	tests := []struct {
		name   string
		before func() *Sampler
	}{
		{
			name: "a sampler that never started has nothing to stop",
			before: func() *Sampler {
				return NewSampler(Options{}, mockPublisher, mockMonitor, mockRegistry)
			},
		},
		{
			name: "a started sampler stops its goroutine",
			before: func() *Sampler {
				sampler := NewSampler(Options{}, mockPublisher, mockMonitor, mockRegistry)

				require.NoError(t, sampler.Start(t.Context()))

				return sampler
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := tt.before()

			err := sampler.Stop(t.Context())

			require.NoError(t, err)
		})
	}
}

func Test_Sampler_Stop_ContextDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPublisher := NewMockPublisher(ctrl)
	mockMonitor := NewMockMonitor(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	sampler := NewSampler(Options{Enabled: true}, mockPublisher, mockMonitor, mockRegistry)

	release := make(chan struct{})
	priming := func(context.Context, int) (Stats, error) {
		<-release

		return Stats{}, nil
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	mockMonitor.EXPECT().GetStats(gomock.Any(), os.Getpid()).DoAndReturn(priming)

	require.NoError(t, sampler.Start(t.Context()))

	err := sampler.Stop(ctx)

	close(release)
	<-sampler.done

	require.ErrorIs(t, err, context.Canceled)
}
