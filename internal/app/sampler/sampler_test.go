package sampler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
)

func Test_NewSampler(t *testing.T) {
	s := NewSampler(bus.NoOp(), monitor.NewMonitor())

	assert.NotNil(t, s)
}

func Test_Sampler_Run_StopsOnContextCancel(t *testing.T) {
	s := NewSampler(bus.NoOp(), monitor.NewMonitor())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sampler did not stop after context cancel")
	}
}

func Test_Sampler_Sample_PublishesEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockBus := bus.NewMockBus(ctrl)

	mockMonitor.EXPECT().
		GetStats(gomock.Any(), gomock.Any()).
		Return(monitor.Stats{CPU: 2.5, MEM: 64.0}, nil)

	mockBus.EXPECT().
		Publish(gomock.Any()).
		Do(func(msg bus.Message) {
			assert.Equal(t, bus.EventResourceSample, msg.Type)

			data, ok := msg.Data.(bus.ResourceSample)
			assert.True(t, ok)
			assert.InDelta(t, 2.5, data.CPU, 0.001)
			assert.InDelta(t, 64.0, data.MEM, 0.001)
		})

	s := &sampler{
		bus:     mockBus,
		monitor: mockMonitor,
	}

	s.sample(context.Background())
}

func Test_Sampler_Sample_SkipsOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockBus := bus.NewMockBus(ctrl)

	mockMonitor.EXPECT().
		GetStats(gomock.Any(), gomock.Any()).
		Return(monitor.Stats{}, assert.AnError)

	s := &sampler{
		bus:     mockBus,
		monitor: mockMonitor,
	}

	s.sample(context.Background())
}
