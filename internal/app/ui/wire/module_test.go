package wire

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/monitor"
	"fuku/internal/app/runtime"
	"fuku/internal/app/ui"
	"fuku/internal/app/ui/logs"
	"fuku/internal/app/ui/navigation"
	"fuku/internal/app/ui/services"
	"fuku/internal/config/logger"
)

func Test_NewUI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	mockCommandBus := runtime.NewMockCommandBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogView := ui.NewMockLogView(ctrl)
	mockNavigator := navigation.NewMockNavigator(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	sender := logs.NewSender()
	subscriber := logs.NewSubscriber(mockEventBus, sender)

	params := UIParams{
		EventBus:   mockEventBus,
		CommandBus: mockCommandBus,
		Controller: mockController,
		Monitor:    mockMonitor,
		LogView:    mockLogView,
		Navigator:  mockNavigator,
		Loader:     services.NewLoader(),
		Sender:     sender,
		Subscriber: subscriber,
		Logger:     mockLogger,
	}

	factory := NewUI(params)
	assert.NotNil(t, factory)
}

func Test_UI_CreateProgram(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	mockCommandBus := runtime.NewMockCommandBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogView := ui.NewMockLogView(ctrl)
	mockNavigator := navigation.NewMockNavigator(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	ctx := context.Background()
	eventChan := make(chan runtime.Event)
	close(eventChan)

	sender := logs.NewSender()

	mockEventBus.EXPECT().Subscribe(ctx).Return(eventChan)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	mockNavigator.EXPECT().CurrentView().Return(navigation.ViewServices).AnyTimes()

	subscriber := logs.NewSubscriber(mockEventBus, sender)

	params := UIParams{
		EventBus:   mockEventBus,
		CommandBus: mockCommandBus,
		Controller: mockController,
		Monitor:    mockMonitor,
		LogView:    mockLogView,
		Navigator:  mockNavigator,
		Loader:     services.NewLoader(),
		Sender:     sender,
		Subscriber: subscriber,
		Logger:     mockLogger,
	}

	factory := NewUI(params)
	program, err := factory(ctx, "test-profile")

	assert.NoError(t, err)
	assert.NotNil(t, program)
}

func Test_UI_MultipleProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEventBus := runtime.NewMockEventBus(ctrl)
	mockCommandBus := runtime.NewMockCommandBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogView := ui.NewMockLogView(ctrl)
	mockNavigator := navigation.NewMockNavigator(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	tests := []struct {
		name    string
		profile string
	}{
		{name: "Default profile", profile: "default"},
		{name: "Custom profile", profile: "custom"},
		{name: "Empty profile", profile: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			eventChan := make(chan runtime.Event)
			close(eventChan)

			sender := logs.NewSender()

			mockEventBus.EXPECT().Subscribe(ctx).Return(eventChan)
			mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
			mockNavigator.EXPECT().CurrentView().Return(navigation.ViewServices).AnyTimes()

			subscriber := logs.NewSubscriber(mockEventBus, sender)

			params := UIParams{
				EventBus:   mockEventBus,
				CommandBus: mockCommandBus,
				Controller: mockController,
				Monitor:    mockMonitor,
				LogView:    mockLogView,
				Navigator:  mockNavigator,
				Loader:     services.NewLoader(),
				Sender:     sender,
				Subscriber: subscriber,
				Logger:     mockLogger,
			}

			factory := NewUI(params)
			program, err := factory(ctx, tt.profile)

			assert.NoError(t, err)
			assert.NotNil(t, program)
		})
	}
}
