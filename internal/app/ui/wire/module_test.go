package wire

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/monitor"
	"fuku/internal/app/ui/services"
	"fuku/internal/config/logger"
)

func Test_NewUI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)

	params := UIParams{
		Bus:        mockBus,
		Controller: mockController,
		Monitor:    mockMonitor,
		Loader:     services.NewLoader(),
		Logger:     mockLogger,
	}

	factory := NewUI(params)
	assert.NotNil(t, factory)
}

func Test_UI_CreateProgram(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockController := services.NewMockController(ctrl)
	mockMonitor := monitor.NewMockMonitor(ctrl)
	mockLogger := logger.NewMockLogger(ctrl)
	componentLogger := logger.NewMockLogger(ctrl)

	ctx := context.Background()
	msgChan := make(chan bus.Message)
	close(msgChan)

	mockBus.EXPECT().Subscribe(ctx).Return(msgChan)
	mockLogger.EXPECT().WithComponent("UI").Return(componentLogger)
	mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
	componentLogger.EXPECT().Debug().Return(nil).AnyTimes()

	params := UIParams{
		Bus:        mockBus,
		Controller: mockController,
		Monitor:    mockMonitor,
		Loader:     services.NewLoader(),
		Logger:     mockLogger,
	}

	factory := NewUI(params)
	program, err := factory(ctx, "test-profile")

	assert.NoError(t, err)
	assert.NotNil(t, program)
}

func Test_UI_MultipleProfiles(t *testing.T) {
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBus := bus.NewMockBus(ctrl)
			mockController := services.NewMockController(ctrl)
			mockMonitor := monitor.NewMockMonitor(ctrl)
			mockLogger := logger.NewMockLogger(ctrl)
			componentLogger := logger.NewMockLogger(ctrl)

			ctx := context.Background()
			msgChan := make(chan bus.Message)
			close(msgChan)

			mockBus.EXPECT().Subscribe(ctx).Return(msgChan)
			mockLogger.EXPECT().WithComponent("UI").Return(componentLogger)
			mockLogger.EXPECT().Debug().Return(nil).AnyTimes()
			componentLogger.EXPECT().Debug().Return(nil).AnyTimes()

			params := UIParams{
				Bus:        mockBus,
				Controller: mockController,
				Monitor:    mockMonitor,
				Loader:     services.NewLoader(),
				Logger:     mockLogger,
			}

			factory := NewUI(params)
			program, err := factory(ctx, tt.profile)

			assert.NoError(t, err)
			assert.NotNil(t, program)
		})
	}
}
