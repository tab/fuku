package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewLogFilter(t *testing.T) {
	filter := NewLogFilter()
	assert.NotNil(t, filter)
}

func Test_LogFilter_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(f LogFilter)
		service  string
		expected bool
	}{
		{name: "Service not set returns false", setup: func(f LogFilter) {}, service: "api", expected: false},
		{name: "Service enabled returns true", setup: func(f LogFilter) { f.Set("api", true) }, service: "api", expected: true},
		{name: "Service disabled returns false", setup: func(f LogFilter) { f.Set("api", false) }, service: "api", expected: false},
		{name: "Different service returns false", setup: func(f LogFilter) { f.Set("web", true) }, service: "api", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewLogFilter()
			tt.setup(filter)
			result := filter.IsEnabled(tt.service)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_LogFilter_Set(t *testing.T) {
	tests := []struct {
		name    string
		service string
		enabled bool
	}{
		{name: "Enable service", service: "api", enabled: true},
		{name: "Disable service", service: "web", enabled: false},
		{name: "Toggle service", service: "db", enabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewLogFilter()
			filter.Set(tt.service, tt.enabled)
			assert.Equal(t, tt.enabled, filter.IsEnabled(tt.service))
		})
	}
}

func Test_LogFilter_EnableAll(t *testing.T) {
	tests := []struct {
		name     string
		services []string
	}{
		{name: "Enable single service", services: []string{"api"}},
		{name: "Enable multiple services", services: []string{"api", "web", "db"}},
		{name: "Enable empty list", services: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewLogFilter()
			filter.EnableAll(tt.services)

			for _, svc := range tt.services {
				assert.True(t, filter.IsEnabled(svc))
			}
		})
	}
}

func Test_LogFilter_ThreadSafety(t *testing.T) {
	filter := NewLogFilter()
	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				filter.Set("service", true)
				filter.IsEnabled("service")
			}

			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
