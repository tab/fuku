package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"fuku/internal/contracts"
)

func Test_Run(t *testing.T) {
	failure := errors.New("bus overloaded")
	commandErr := errors.New("profile not found: nope")

	tests := []struct {
		name           string
		option         func() fx.Option
		expectedCode   int
		expectedStderr string
	}{
		{
			name: "returns the code the command completed with",
			option: func() fx.Option {
				return fx.Invoke(func(a *Arbiter) {
					go a.decide(3, nil)
				})
			},
			expectedCode: 3,
		},
		{
			name: "a command that completed with an error prints the cause once",
			option: func() fx.Option {
				return fx.Invoke(func(a *Arbiter) {
					go a.decide(1, commandErr)
				})
			},
			expectedCode:   1,
			expectedStderr: "Error: profile not found: nope\n",
		},
		{
			name: "a runtime failure exits 1 and prints the cause after the stop",
			option: func() fx.Option {
				return fx.Invoke(func(a *Arbiter) {
					go a.Fail(failure)
				})
			},
			expectedCode:   1,
			expectedStderr: "Error: bus overloaded\n",
		},
		{
			name: "a start error exits 1 with the error",
			option: func() fx.Option {
				return fx.Invoke(func(lc fx.Lifecycle) {
					lc.Append(fx.Hook{OnStart: func(context.Context) error { return contracts.ErrNoServicesDefined }})
				})
			},
			expectedCode:   1,
			expectedStderr: "Error: no services defined\n",
		},
		{
			name: "a constructor error exits 1 with its root cause",
			option: func() fx.Option {
				return fx.Invoke(func(int) {})
			},
			expectedCode:   1,
			expectedStderr: "Error: missing type: int\n",
		},
		{
			name: "a duplicate instance exits 1 silently",
			option: func() fx.Option {
				return fx.Invoke(func(lc fx.Lifecycle) {
					lc.Append(fx.Hook{OnStart: func(context.Context) error { return contracts.ErrInstanceAlreadyRunning }})
				})
			},
			expectedCode: 1,
		},
		{
			name: "a signal during the start stops the run",
			option: func() fx.Option {
				return fx.Invoke(func(lc fx.Lifecycle) {
					lc.Append(fx.Hook{OnStart: func(context.Context) error {
						return syscall.Kill(os.Getpid(), syscall.SIGTERM)
					}})
				})
			},
			expectedCode: 0,
		},
		{
			name: "a stop error exits 1 with the error",
			option: func() fx.Option {
				return fx.Invoke(func(lc fx.Lifecycle, a *Arbiter) {
					lc.Append(fx.Hook{OnStop: func(context.Context) error { return contracts.ErrNoServicesDefined }})

					go a.decide(0, nil)
				})
			},
			expectedCode:   1,
			expectedStderr: "Error: no services defined\n",
		},
		{
			name: "a stop error prints the recorded cause before it",
			option: func() fx.Option {
				return fx.Invoke(func(lc fx.Lifecycle, a *Arbiter) {
					lc.Append(fx.Hook{OnStop: func(context.Context) error { return context.DeadlineExceeded }})

					go a.Fail(failure)
				})
			},
			expectedCode:   1,
			expectedStderr: "Error: bus overloaded\nError: context deadline exceeded\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arbiter *Arbiter

			stderr := &bytes.Buffer{}
			application := fx.New(fx.NopLogger, fx.Provide(NewArbiter), fx.Populate(&arbiter), tt.option())
			require.NotNil(t, arbiter)

			code := Run(application, arbiter, stderr)

			assert.Equal(t, tt.expectedCode, code)
			assert.Equal(t, tt.expectedStderr, stderr.String())
		})
	}
}
