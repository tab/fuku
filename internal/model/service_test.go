package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Service_SettledSince(t *testing.T) {
	since := time.Now()
	before := since.Add(-time.Second)

	tests := []struct {
		name    string
		service Service
		want    bool
	}{
		{name: "running after since", service: Service{Status: StatusRunning, LifecycleAt: since}, want: true},
		{name: "running before since", service: Service{Status: StatusRunning, LifecycleAt: before}, want: false},
		{name: "starting", service: Service{Status: StatusStarting, LifecycleAt: since}, want: false},
		{name: "failed after since", service: Service{Status: StatusFailed, LifecycleAt: since}, want: true},
		{name: "stopped after since", service: Service{Status: StatusStopped, LifecycleAt: since}, want: true},
		{name: "watched running before the watcher starts", service: Service{Status: StatusRunning, Watch: &Watch{}, LifecycleAt: since}, want: false},
		{name: "watched running and watching", service: Service{Status: StatusRunning, Watch: &Watch{}, Watching: true, LifecycleAt: since}, want: true},
		{name: "watched stopped", service: Service{Status: StatusStopped, Watch: &Watch{}, LifecycleAt: since}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.service.SettledSince(since))
		})
	}
}
