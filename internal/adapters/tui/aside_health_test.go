package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_AsideHealthTab(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		before      func() Model
		service     *model.Service
		wantContain []string
		wantMissing []string
	}{
		{
			name: "http readiness, running pid, retry config, last event",
			before: func() Model {
				m := Model{theme: theme, retryAttempts: 3, retryBackoff: 500 * time.Millisecond}
				m.state.asideTab = AsideTabHealth
				m.state.now = now

				return m
			},
			service: &model.Service{
				ID:        "api",
				Name:      "api",
				Directory: "services/api",
				Readiness: &model.Readiness{
					Type:     model.ReadinessHTTP,
					URL:      "http://localhost:8080/health",
					Interval: 500 * time.Millisecond,
					Timeout:  30 * time.Second,
				},
				Status: model.StatusRunning,
				Process: model.Process{
					PID:       76758,
					StartedAt: now.Add(-4*time.Minute - 9*time.Second),
				},
				LifecycleAt: now.Add(-4*time.Minute - 9*time.Second),
			},
			wantContain: []string{
				"probe",
				"type", "http",
				"url", "http://localhost:8080/health",
				"interval", "500ms",
				"timeout", "30s",
				"process",
				"pid", "76758",
				"uptime", "04:09",
				"retry",
				"attempts", "3",
				"backoff", "500ms",
				"status",
				"state", "running",
				"duration", "04:09",
			},
		},
		{
			name: "tcp readiness uses address",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabHealth
				m.state.now = now

				return m
			},
			service: &model.Service{
				ID:        "redis",
				Name:      "redis",
				Directory: "services/redis",
				Readiness: &model.Readiness{
					Type:    model.ReadinessTCP,
					Address: "localhost:6379",
				},
				Status: model.StatusRunning,
			},
			wantContain: []string{
				"probe",
				"address",
				"localhost:6379",
			},
			wantMissing: []string{
				"url",
				"pattern",
			},
		},
		{
			name: "log readiness uses pattern",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabHealth
				m.state.now = now

				return m
			},
			service: &model.Service{
				ID:        "worker",
				Name:      "worker",
				Directory: "services/worker",
				Readiness: &model.Readiness{
					Type:    model.ReadinessLog,
					Pattern: "ready",
				},
				Status: model.StatusRunning,
			},
			wantContain: []string{
				"probe",
				"pattern",
				"ready",
			},
		},
		{
			name: "no readiness omits probe card",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabHealth
				m.state.now = now

				return m
			},
			service: &model.Service{
				ID:        "web",
				Name:      "web",
				Directory: "services/web",
				Status:    model.StatusRunning,
			},
			wantMissing: []string{
				"probe",
			},
		},
		{
			name: "stopped service still shows process card with placeholder rows so the layout does not flicker",
			before: func() Model {
				m := Model{theme: theme}
				m.state.asideTab = AsideTabHealth
				m.state.now = now

				return m
			},
			service: &model.Service{
				ID:        "web",
				Name:      "web",
				Directory: "services/web",
				Status:    model.StatusStopped,
			},
			wantContain: []string{
				"status",
				"state",
				"stopped",
				"process",
				"pid",
				"uptime",
				asidePlaceholder,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.before()

			result := m.asideContent(tt.service, 80)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want)
			}

			for _, miss := range tt.wantMissing {
				assert.NotContains(t, result, miss)
			}
		})
	}
}

func Test_FormatElapsed(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{
			name: "negative duration clamps to zero",
			in:   -5 * time.Second,
			want: "00:00",
		},
		{
			name: "seconds only",
			in:   9 * time.Second,
			want: "00:09",
		},
		{
			name: "minutes and seconds",
			in:   4*time.Minute + 9*time.Second,
			want: "04:09",
		},
		{
			name: "hours wraps",
			in:   2*time.Hour + 4*time.Minute + 9*time.Second,
			want: "02:04:09",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatElapsed(tt.in)

			assert.Equal(t, tt.want, result)
		})
	}
}
