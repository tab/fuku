package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/terminal"
	"fuku/internal/model"
)

func Test_UpdateBlinkAnimations(t *testing.T) {
	activeBlink := terminal.NewBlink()
	activeBlink.Start()

	tests := []struct {
		name               string
		views              map[string]*serviceView
		status             model.Status
		expectedHasActive  bool
		expectBlinkStarted map[string]bool
	}{
		{
			name:              "no services",
			views:             map[string]*serviceView{},
			expectedHasActive: false,
		},
		{
			name: "running service stops blink",
			views: map[string]*serviceView{
				"api": {Blink: activeBlink},
			},
			status:             model.StatusRunning,
			expectedHasActive:  false,
			expectBlinkStarted: map[string]bool{"api": false},
		},
		{
			name: "starting service activates blink",
			views: map[string]*serviceView{
				"api": {Blink: terminal.NewBlink()},
			},
			status:             model.StatusStarting,
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "stopping service activates blink",
			views: map[string]*serviceView{
				"api": {Blink: terminal.NewBlink()},
			},
			status:             model.StatusStopping,
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "restarting service activates blink",
			views: map[string]*serviceView{
				"api": {Blink: terminal.NewBlink()},
			},
			status:             model.StatusRestarting,
			expectedHasActive:  true,
			expectBlinkStarted: map[string]bool{"api": true},
		},
		{
			name: "pending service does not blink",
			views: map[string]*serviceView{
				"api": {Blink: terminal.NewBlink()},
			},
			status:             model.StatusPending,
			expectedHasActive:  false,
			expectBlinkStarted: map[string]bool{"api": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{snapshot: &model.Snapshot{Services: map[string]*model.Service{"api": {Name: "api", Status: tt.status}}}}
			m.state.views = tt.views

			result := m.updateBlinkAnimations()

			assert.Equal(t, tt.expectedHasActive, result)

			for id, expectStarted := range tt.expectBlinkStarted {
				assert.Equal(t, expectStarted, m.state.views[id].Blink.IsActive())
			}
		})
	}
}
