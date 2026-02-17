package services

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
)

func Test_Loader_Start(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Loader)
		service    string
		msg        string
		wantLen    int
		wantMsg    string
		wantActive bool
	}{
		{
			name:       "adds first service",
			setup:      func(l *Loader) {},
			service:    "api",
			msg:        "starting api…",
			wantLen:    1,
			wantMsg:    "starting api…",
			wantActive: true,
		},
		{
			name: "adds second service",
			setup: func(l *Loader) {
				l.Start("storage", "starting storage…")
			},
			service:    "api",
			msg:        "starting api…",
			wantLen:    2,
			wantMsg:    "starting storage…",
			wantActive: true,
		},
		{
			name: "updates existing service message",
			setup: func(l *Loader) {
				l.Start("api", "starting api…")
			},
			service:    "api",
			msg:        "restarting api…",
			wantLen:    1,
			wantMsg:    "restarting api…",
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
			tt.setup(loader)
			loader.Start(tt.service, tt.msg)

			assert.Equal(t, tt.wantLen, len(loader.queue))
			assert.Equal(t, tt.wantMsg, loader.Message())
			assert.Equal(t, tt.wantActive, loader.Active)
		})
	}
}

func Test_Loader_Stop(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Loader)
		service    string
		wantLen    int
		wantMsg    string
		wantActive bool
	}{
		{
			name: "removes only service",
			setup: func(l *Loader) {
				l.Start("api", "starting api…")
			},
			service:    "api",
			wantLen:    0,
			wantMsg:    "",
			wantActive: false,
		},
		{
			name: "removes first service shows second",
			setup: func(l *Loader) {
				l.Start("storage", "starting storage…")
				l.Start("api", "starting api…")
			},
			service:    "storage",
			wantLen:    1,
			wantMsg:    "starting api…",
			wantActive: true,
		},
		{
			name: "removes second service shows first",
			setup: func(l *Loader) {
				l.Start("storage", "starting storage…")
				l.Start("api", "starting api…")
			},
			service:    "api",
			wantLen:    1,
			wantMsg:    "starting storage…",
			wantActive: true,
		},
		{
			name: "removes middle service",
			setup: func(l *Loader) {
				l.Start("storage", "starting storage…")
				l.Start("api", "starting api…")
				l.Start("user", "starting user…")
			},
			service:    "api",
			wantLen:    2,
			wantMsg:    "starting storage…",
			wantActive: true,
		},
		{
			name:       "handles non-existent service",
			setup:      func(l *Loader) {},
			service:    "api",
			wantLen:    0,
			wantMsg:    "",
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
			tt.setup(loader)
			loader.Stop(tt.service)

			assert.Equal(t, tt.wantLen, len(loader.queue))
			assert.Equal(t, tt.wantMsg, loader.Message())
			assert.Equal(t, tt.wantActive, loader.Active)
		})
	}
}

func Test_Loader_StopAll(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
	loader.Start("storage", "starting storage…")
	loader.Start("api", "starting api…")
	loader.Start("user", "starting user…")

	assert.Equal(t, 3, len(loader.queue))
	assert.True(t, loader.Active)

	loader.StopAll()

	assert.Equal(t, 0, len(loader.queue))
	assert.False(t, loader.Active)
	assert.Equal(t, "", loader.Message())
}

func Test_Loader_Message(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Loader)
		wantMsg string
	}{
		{
			name:    "empty queue returns empty string",
			setup:   func(l *Loader) {},
			wantMsg: "",
		},
		{
			name: "returns first message in queue (FIFO)",
			setup: func(l *Loader) {
				l.Start("storage", "starting storage…")
				l.Start("api", "starting api…")
			},
			wantMsg: "starting storage…",
		},
		{
			name: "single item returns its message",
			setup: func(l *Loader) {
				l.Start("api", "restarting api…")
			},
			wantMsg: "restarting api…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
			tt.setup(loader)
			assert.Equal(t, tt.wantMsg, loader.Message())
		})
	}
}

func Test_Loader_FIFO_Order(t *testing.T) {
	loader := &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}

	loader.Start("storage", "starting storage…")
	loader.Start("api", "starting api…")
	loader.Start("user", "starting user…")

	assert.Equal(t, "starting storage…", loader.Message())

	loader.Stop("storage")
	assert.Equal(t, "starting api…", loader.Message())

	loader.Stop("api")
	assert.Equal(t, "starting user…", loader.Message())

	loader.Stop("user")
	assert.Equal(t, "", loader.Message())
	assert.False(t, loader.Active)
}
