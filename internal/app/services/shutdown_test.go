package services

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_Runtime_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)

	log := slog.New(slog.DiscardHandler)

	runtime := NewRuntime(RuntimeParams{Guard: NewGuard(mockTracker), Logger: log})

	tests := []struct {
		name     string
		before   func() context.Context
		expected error
	}{
		{
			name: "does nothing without an active run",
			before: func() context.Context {
				return t.Context()
			},
		},
		{
			name: "cancels the run with the StopAll cause before the profile is resolved",
			before: func() context.Context {
				return runtime.begin(t.Context())
			},
			expected: errStopAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.before()

			runtime.stopAll()

			require.ErrorIs(t, context.Cause(ctx), tt.expected)
		})
	}
}

func Test_Runtime_Shutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracker := NewMockTracker(ctrl)
	mockPublisher := NewMockPublisher(ctrl)
	mockFirst := NewMockProcess(ctrl)
	mockSecond := NewMockProcess(ctrl)

	log := slog.New(slog.DiscardHandler)

	first := model.Service{ID: "test-id-postgres", Name: "postgres", Tier: "foundation"}
	second := model.Service{ID: "test-id-api", Name: "api", Tier: "platform"}
	tiers := []model.Tier{
		{Name: "foundation", Services: []*model.Service{&first}},
		{Name: "platform", Services: []*model.Service{&second}},
	}
	firstDone := make(chan struct{})
	secondDone := make(chan struct{})
	terminateFirst := func() error {
		close(firstDone)

		return nil
	}
	terminateSecond := func() error {
		close(secondDone)

		return nil
	}

	var joined atomic.Bool

	waitForCancel := func(ctx context.Context) {
		<-ctx.Done()
		joined.Store(true)
	}

	isEvent := func(msgType contracts.MessageType, svc model.Service) gomock.Matcher {
		return gomock.Cond(func(msg contracts.Message) bool {
			var id string

			switch data := msg.Data.(type) {
			case contracts.ServiceStopping:
				id = data.Service.ID
			case contracts.ServiceStopped:
				id = data.Service.ID
			}

			return msg.Type == msgType && id == svc.ID
		})
	}

	runtime := NewRuntime(RuntimeParams{Tracker: mockTracker, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Logger: log})

	runtime.begin(t.Context())
	runtime.guard.resolve(tiers)
	runtime.guard.dispatch(second.ID)
	runtime.dispatch(second.ID, waitForCancel)

	mockFirst.EXPECT().Service().Return(first).AnyTimes()
	mockFirst.EXPECT().Done().Return(firstDone)
	mockSecond.EXPECT().Service().Return(second).AnyTimes()
	mockSecond.EXPECT().Done().Return(secondDone)
	mockTracker.EXPECT().Reverse().Return([]contracts.Process{mockSecond, mockFirst})
	mockTracker.EXPECT().Detach(second.ID).Times(2)
	mockTracker.EXPECT().Detach(first.ID).Times(2)
	mockTracker.EXPECT().Get(second.ID).Return(mockSecond, true)
	mockTracker.EXPECT().Get(first.ID).Return(mockFirst, true)
	mockTracker.EXPECT().Untrack(second.ID, mockSecond).Return(false)
	mockTracker.EXPECT().Untrack(first.ID, mockFirst).Return(false)
	gomock.InOrder(
		mockPublisher.EXPECT().Publish(isEvent(contracts.EventServiceStopping, second)).Return(nil),
		mockSecond.EXPECT().Terminate().DoAndReturn(terminateSecond),
		mockPublisher.EXPECT().Publish(isEvent(contracts.EventServiceStopped, second)).Return(nil),
		mockPublisher.EXPECT().Publish(isEvent(contracts.EventServiceStopping, first)).Return(nil),
		mockFirst.EXPECT().Terminate().DoAndReturn(terminateFirst),
		mockPublisher.EXPECT().Publish(isEvent(contracts.EventServiceStopped, first)).Return(nil),
	)

	count := runtime.shutdown()

	assert.Equal(t, 2, count)
	assert.True(t, joined.Load())
	assert.Nil(t, runtime.work)
}

func Test_Runtime_Shutdown_StopsATierConcurrently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockTracker := NewMockTracker(ctrl)
		mockPublisher := NewMockPublisher(ctrl)
		mockPostgres := NewMockProcess(ctrl)
		mockRedis := NewMockProcess(ctrl)

		log := slog.New(slog.DiscardHandler)

		postgres := model.Service{ID: "test-id-postgres", Name: "postgres", Tier: "foundation"}
		redis := model.Service{ID: "test-id-redis", Name: "redis", Tier: "foundation"}
		postgresDone := make(chan struct{})
		redisDone := make(chan struct{})

		var entered sync.WaitGroup

		entered.Add(2)

		terminateTogether := func(done chan struct{}) func() error {
			return func() error {
				entered.Done()
				entered.Wait()
				close(done)

				return nil
			}
		}

		runtime := NewRuntime(RuntimeParams{Tracker: mockTracker, Guard: NewGuard(mockTracker), Publisher: mockPublisher, Logger: log})

		runtime.begin(t.Context())

		mockPostgres.EXPECT().Service().Return(postgres).AnyTimes()
		mockPostgres.EXPECT().Done().Return(postgresDone)
		mockPostgres.EXPECT().Terminate().DoAndReturn(terminateTogether(postgresDone))
		mockRedis.EXPECT().Service().Return(redis).AnyTimes()
		mockRedis.EXPECT().Done().Return(redisDone)
		mockRedis.EXPECT().Terminate().DoAndReturn(terminateTogether(redisDone))
		mockTracker.EXPECT().Reverse().Return([]contracts.Process{mockRedis, mockPostgres})
		mockTracker.EXPECT().Detach(redis.ID).Times(2)
		mockTracker.EXPECT().Detach(postgres.ID).Times(2)
		mockTracker.EXPECT().Get(redis.ID).Return(mockRedis, true)
		mockTracker.EXPECT().Get(postgres.ID).Return(mockPostgres, true)
		mockTracker.EXPECT().Untrack(redis.ID, mockRedis).Return(false)
		mockTracker.EXPECT().Untrack(postgres.ID, mockPostgres).Return(false)
		mockPublisher.EXPECT().Publish(gomock.Any()).Return(nil).Times(4)

		count := runtime.shutdown()

		assert.Equal(t, 2, count)
	})
}
