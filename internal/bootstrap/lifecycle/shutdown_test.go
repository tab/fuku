package lifecycle

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"
)

func Test_Arbiter_FirstOutcomeWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShutdowner := NewMockShutdowner(ctrl)

	failure := errors.New("bus overloaded")
	commandErr := errors.New("profile not found: nope")

	tests := []struct {
		name         string
		before       func() *Arbiter
		expectedCode int
		expectedErr  error
		expectedSig  os.Signal
	}{
		{
			name: "a failure recorded first keeps its code over a later completion",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).Return(nil)

				arbiter.Fail(failure)
				arbiter.decide(0, nil)

				return arbiter
			},
			expectedCode: 1,
			expectedErr:  failure,
		},
		{
			name: "a completion recorded first keeps its code over a later failure",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).Return(nil)

				arbiter.decide(3, nil)
				arbiter.Fail(failure)

				return arbiter
			},
			expectedCode: 3,
		},
		{
			name: "a completion with an error keeps the error as the cause",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).Return(nil)

				arbiter.decide(1, commandErr)

				return arbiter
			},
			expectedCode: 1,
			expectedErr:  commandErr,
		},
		{
			name: "a signal before any outcome is announced and keeps exit code 0",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				arbiter.observe(syscall.SIGTERM)

				return arbiter
			},
			expectedSig: syscall.SIGTERM,
		},
		{
			name: "a signal after the outcome is ignored",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				mockShutdowner.EXPECT().Shutdown(gomock.Len(1)).Return(nil)

				arbiter.decide(0, nil)
				arbiter.observe(syscall.SIGTERM)

				return arbiter
			},
		},
		{
			name: "a signal recorded first keeps exit code 0 over a later cancellation result",
			before: func() *Arbiter {
				arbiter := NewArbiter(mockShutdowner)

				arbiter.observe(syscall.SIGINT)
				arbiter.decide(1, nil)
				arbiter.Fail(failure)

				return arbiter
			},
			expectedSig: syscall.SIGINT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arbiter := tt.before()

			code, cause := arbiter.outcome()

			require.ErrorIs(t, cause, tt.expectedErr)
			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedSig, arbiter.signal())
		})
	}
}

func Test_Arbiter_decide_StopsTheContainerWithTheCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockShutdowner := NewMockShutdowner(ctrl)

	arbiter := NewArbiter(mockShutdowner)
	exitCode := func(options ...fx.ShutdownOption) error {
		assert.Len(t, options, 1)
		assert.Equal(t, fx.ExitCode(2), options[0])

		return nil
	}

	mockShutdowner.EXPECT().Shutdown(gomock.Any()).DoAndReturn(exitCode)

	arbiter.decide(2, nil)
}
