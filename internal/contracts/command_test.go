package contracts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Action_Command(t *testing.T) {
	tests := []struct {
		name     string
		action   Action
		expected MessageType
	}{
		{
			name:     "start",
			action:   ActionStart,
			expected: CommandStartService,
		},
		{
			name:     "stop",
			action:   ActionStop,
			expected: CommandStopService,
		},
		{
			name:     "restart",
			action:   ActionRestart,
			expected: CommandRestartService,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.Command())
		})
	}
}
