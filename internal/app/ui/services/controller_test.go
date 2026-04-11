package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
)

func Test_NewController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)

	c := NewController(mockBus)

	assert.NotNil(t, c)
	impl, ok := c.(*controller)
	assert.True(t, ok)
	assert.Equal(t, mockBus, impl.bus)
}

func Test_Controller_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)

	c := NewController(mockBus)

	svc := bus.Service{ID: "test-id-api", Name: "api"}

	mockBus.EXPECT().Publish(bus.Message{
		Type:     bus.CommandStartService,
		Data:     svc,
		Critical: true,
	})

	c.Start(svc)
}

func Test_Controller_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)

	c := NewController(mockBus)

	svc := bus.Service{ID: "test-id-api", Name: "api"}

	mockBus.EXPECT().Publish(bus.Message{
		Type:     bus.CommandStopService,
		Data:     svc,
		Critical: true,
	})

	c.Stop(svc)
}

func Test_Controller_Restart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)

	c := NewController(mockBus)

	svc := bus.Service{ID: "test-id-api", Name: "api"}

	mockBus.EXPECT().Publish(bus.Message{
		Type:     bus.CommandRestartService,
		Data:     svc,
		Critical: true,
	})

	c.Restart(svc)
}

func Test_Controller_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)

	c := NewController(mockBus)

	mockBus.EXPECT().Publish(bus.Message{Type: bus.CommandStopAll, Critical: true})

	c.StopAll()
}
