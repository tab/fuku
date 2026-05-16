package wire

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/api"
	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/app/registry"
	"fuku/internal/app/ui/services"
	"fuku/internal/app/updater"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewUI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	mockChecker := updater.NewMockChecker(ctrl)

	params := UIParams{
		Config:     &config.Config{},
		API:        api.NewMockListener(ctrl),
		Bus:        mockBus,
		Controller: mockController,
		Store:      mockStore,
		Monitor:    mockMonitor,
		Loader:     services.NewLoader(),
		Logger:     mockLogger,
		Checker:    mockChecker,
	}

	factory := NewUI(params)
	assert.NotNil(t, factory)
}

func Test_UI_CreateProgram(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockChecker := updater.NewMockChecker(ctrl)

	ctx := context.Background()
	msgChan := make(chan bus.Message)
	close(msgChan)

	mockAPI := api.NewMockListener(ctrl)

	subscribed := make(chan struct{})
	checkerStarted := make(chan struct{})

	mockBus.EXPECT().Subscribe(ctx).DoAndReturn(func(_ context.Context) <-chan bus.Message {
		close(subscribed)
		return msgChan
	})
	mockChecker.EXPECT().Run(ctx).Do(func(_ context.Context) {
		select {
		case <-subscribed:
		default:
			t.Errorf("checker.Run called before bus.Subscribe")
		}

		close(checkerStarted)
	})
	mockLogger.EXPECT().WithComponent("UI").Return(componentLogger)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	componentLogger.EXPECT().Debug().Return(nil).AnyTimes()

	params := UIParams{
		Config:     &config.Config{},
		API:        mockAPI,
		Bus:        mockBus,
		Controller: mockController,
		Store:      mockStore,
		Monitor:    mockMonitor,
		Loader:     services.NewLoader(),
		Logger:     mockLogger,
		Checker:    mockChecker,
	}

	factory := NewUI(params)
	program, err := factory(ctx, "test-profile")

	require.NoError(t, err)
	assert.NotNil(t, program)

	select {
	case <-checkerStarted:
	case <-time.After(time.Second):
		t.Fatal("checker.Run was not invoked")
	}
}

func Test_UI_MultipleProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockStore := registry.NewMockStore(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)
	mockChecker := updater.NewMockChecker(ctrl)

	ctx := context.Background()

	tests := []struct {
		name    string
		profile string
	}{
		{
			name:    "Default profile",
			profile: "default",
		},
		{
			name:    "Custom profile",
			profile: "custom",
		},
		{
			name:    "Empty profile",
			profile: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgChan := make(chan bus.Message)
			close(msgChan)

			mockAPI := api.NewMockListener(ctrl)

			mockBus.EXPECT().Subscribe(ctx).Return(msgChan)
			mockChecker.EXPECT().Run(ctx).AnyTimes()
			mockLogger.EXPECT().WithComponent("UI").Return(componentLogger)
			mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
			componentLogger.EXPECT().Debug().Return(nil).AnyTimes()

			params := UIParams{
				Config:     &config.Config{},
				API:        mockAPI,
				Bus:        mockBus,
				Controller: mockController,
				Store:      mockStore,
				Monitor:    mockMonitor,
				Loader:     services.NewLoader(),
				Logger:     mockLogger,
				Checker:    mockChecker,
			}

			factory := NewUI(params)
			program, err := factory(ctx, tt.profile)

			require.NoError(t, err)
			assert.NotNil(t, program)
		})
	}
}
