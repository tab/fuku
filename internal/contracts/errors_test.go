package contracts

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ActionNotAllowedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected Action
	}{
		{
			name:     "bare",
			err:      ActionNotAllowedError{Action: ActionStart},
			expected: ActionStart,
		},
		{
			name:     "wrapped",
			err:      fmt.Errorf("admission: %w", ActionNotAllowedError{Action: ActionRestart}),
			expected: ActionRestart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.err, ErrActionNotAllowed)

			var target ActionNotAllowedError

			require.ErrorAs(t, tt.err, &target)
			assert.Equal(t, tt.expected, target.Action)
			assert.Contains(t, tt.err.Error(), string(tt.expected))
		})
	}
}

func Test_ActionNotAllowedError_OtherSentinel(t *testing.T) {
	assert.NotErrorIs(t, ActionNotAllowedError{Action: ActionStop}, ErrServiceBusy)
}
