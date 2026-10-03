package tui

import (
	"bytes"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/adapters/detach"
	"fuku/internal/adapters/terminal"
)

func Test_startupModel_Update(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)
	opened := time.Now().Add(-3 * time.Second)

	profile := detach.Record{Kind: detach.KindProfile, Services: []string{"postgres", "api", "worker"}}

	tests := []struct {
		name     string
		records  []detach.Record
		expected []string
	}{
		{
			name:     "lists every service as waiting once the profile resolves",
			records:  []detach.Record{profile},
			expected: []string{"[+] fuku run core -d 0/3", "postgres  Waiting", "api       Waiting", "worker    Waiting"},
		},
		{
			name: "moves each service through its states",
			records: []detach.Record{
				profile,
				{Kind: detach.KindReady, Service: "postgres", Duration: 1200 * time.Millisecond},
				{Kind: detach.KindStarting, Service: "api"},
				{Kind: detach.KindRunning, PID: 42},
			},
			expected: []string{
				"[+] fuku run core -d 1/3",
				" ✔ postgres  Ready      1.2s",
				"api       Starting",
				"worker    Waiting",
			},
		},
		{
			name: "a failure stops every service still waiting",
			records: []detach.Record{
				profile,
				{Kind: detach.KindReady, Service: "postgres", Duration: 1200 * time.Millisecond},
				{Kind: detach.KindFailed, Service: "worker", Error: "max retries exceeded"},
			},
			expected: []string{
				"[+] fuku run core -d 1/3",
				" ✔ postgres  Ready      1.2s",
				" ○ api       Stopped    3.0s",
				" ✗ worker    Failed     3.0s",
			},
		},
		{
			name: "a failure freezes a starting service as stopped",
			records: []detach.Record{
				profile,
				{Kind: detach.KindStarting, Service: "api"},
				{Kind: detach.KindFailed, Service: "worker", Error: "max retries exceeded"},
			},
			expected: []string{
				"[+] fuku run core -d 0/3",
				" ○ postgres  Stopped    3.0s",
				" ○ api       Stopped    3.0s",
				" ✗ worker    Failed     3.0s",
			},
		},
		{
			name:     "aligns the columns by the display width of non-ASCII names",
			records:  []detach.Record{{Kind: detach.KindProfile, Services: []string{"café", "東京", "api"}}},
			expected: []string{" café  Waiting", " 東京  Waiting", " api   Waiting"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current tea.Model = newStartupModel("core", theme, opened)

			for _, record := range tt.records {
				current, _ = current.Update(record)
			}

			model, ok := current.(startupModel)
			require.True(t, ok)

			model.opened = model.opened.Add(-time.Minute)

			view := ansi.Strip(model.View().Content)

			for _, line := range tt.expected {
				assert.Contains(t, view, line)
			}
		})
	}
}

func Test_startupModel_Init(t *testing.T) {
	model := newStartupModel("core", terminal.NewTheme(terminal.AppearanceDark), time.Now())

	assert.NotNil(t, model.Init())
}

func Test_startupModel_Update_Tick(t *testing.T) {
	model := newStartupModel("core", terminal.NewTheme(terminal.AppearanceDark), time.Now())

	_, cmd := model.Update(model.spinner.Tick())

	assert.IsType(t, spinner.TickMsg{}, cmd())
}

func Test_startupModel_Update_Done(t *testing.T) {
	model := newStartupModel("core", terminal.NewTheme(terminal.AppearanceDark), time.Now())

	_, cmd := model.Update(startupDone{})

	require.NotNil(t, cmd)
	assert.Equal(t, tea.QuitMsg{}, cmd())
}

func Test_startupModel_Update_CtrlC(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	defer signal.Stop(signals)

	model := newStartupModel("core", terminal.NewTheme(terminal.AppearanceDark), time.Now())

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)

	assert.Nil(t, cmd())
	assert.Equal(t, syscall.SIGINT, <-signals)
}

func Test_startupModel_Update_OtherKey(t *testing.T) {
	model := newStartupModel("core", terminal.NewTheme(terminal.AppearanceDark), time.Now())

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'q'})

	assert.Nil(t, cmd)
}

func Test_Startup_OpenShowClose_DoesNotBlock(t *testing.T) {
	theme := terminal.NewTheme(terminal.AppearanceDark)

	var stdout bytes.Buffer

	startup := NewStartup(Options{Profile: "core"}, func() terminal.Theme { return theme }, &stdout)

	startup.Open()
	startup.Show(detach.Record{Kind: detach.KindProfile, Services: []string{"api"}})
	startup.Show(detach.Record{Kind: detach.KindReady, Service: "api", Duration: time.Second})
	startup.Close()

	select {
	case <-startup.done:
	default:
		t.Fatal("the view is still running after Close")
	}
}
