package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/registry"
)

func Test_NewController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)

	c := NewController(mockBus, mockStore)

	assert.NotNil(t, c)
	impl, ok := c.(*controller)
	assert.True(t, ok)
	assert.Equal(t, mockBus, impl.bus)
	assert.Equal(t, mockStore, impl.store)
}

func Test_Controller_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)

	c := NewController(mockBus, mockStore)

	tests := []struct {
		name   string
		before func()
		expect bool
	}{
		{
			name: "not found",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{}, false)
			},
			expect: false,
		},
		{
			name: "running - no-op",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusRunning,
				}, true)
			},
			expect: false,
		},
		{
			name: "stopped - publishes CommandStartService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusStopped,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandStartService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
		{
			name: "failed - publishes CommandStartService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusFailed,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandStartService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			assert.Equal(t, tt.expect, c.Start("api"))
		})
	}
}

func Test_Controller_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)

	c := NewController(mockBus, mockStore)

	tests := []struct {
		name   string
		before func()
		expect bool
	}{
		{
			name: "not found",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{}, false)
			},
			expect: false,
		},
		{
			name: "stopped - no-op",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusStopped,
				}, true)
			},
			expect: false,
		},
		{
			name: "running - publishes CommandStopService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusRunning,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandStopService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			assert.Equal(t, tt.expect, c.Stop("api"))
		})
	}
}

func Test_Controller_Restart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)

	c := NewController(mockBus, mockStore)

	tests := []struct {
		name   string
		before func()
		expect bool
	}{
		{
			name: "not found",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{}, false)
			},
			expect: false,
		},
		{
			name: "starting - no-op",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusStarting,
				}, true)
			},
			expect: false,
		},
		{
			name: "running - publishes CommandRestartService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusRunning,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandRestartService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
		{
			name: "failed - publishes CommandRestartService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusFailed,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandRestartService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
		{
			name: "stopped - publishes CommandRestartService",
			before: func() {
				mockStore.EXPECT().Service("api").Return(registry.ServiceSnapshot{
					Name:   "api",
					Status: registry.StatusStopped,
				}, true)
				mockBus.EXPECT().Publish(bus.Message{
					Type: bus.CommandRestartService,
					Data: bus.Payload{Name: "api"},
				})
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()
			assert.Equal(t, tt.expect, c.Restart("api"))
		})
	}
}

func Test_Controller_StopAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := bus.NewMockBus(ctrl)
	mockStore := registry.NewMockStore(ctrl)

	c := NewController(mockBus, mockStore)

	mockBus.EXPECT().Publish(bus.Message{Type: bus.CommandStopAll})

	c.StopAll()
}
