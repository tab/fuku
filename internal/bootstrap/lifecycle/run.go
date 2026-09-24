package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.uber.org/dig"
	"go.uber.org/fx"

	"fuku/internal/contracts"
)

// Run starts the container, waits for the first terminal outcome, stops it and returns the exit code
func Run(app *fx.App, arbiter *Arbiter, stderr io.Writer) int {
	startCtx, cancelStart := context.WithTimeout(context.Background(), app.StartTimeout())
	defer cancelStart()

	err := app.Start(startCtx)

	if errors.Is(err, contracts.ErrInstanceAlreadyRunning) {
		return 1
	}

	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", dig.RootCause(err))

		return 1
	}

	sig := <-app.Wait()
	arbiter.observe(sig.Signal)

	stopCtx, cancelStop := context.WithTimeout(context.Background(), app.StopTimeout())
	defer cancelStop()

	stopErr := app.Stop(stopCtx)

	code, cause := arbiter.outcome()
	if cause != nil {
		fmt.Fprintf(stderr, "Error: %v\n", cause)
	}

	if stopErr != nil {
		fmt.Fprintf(stderr, "Error: %v\n", stopErr)

		return 1
	}

	return code
}
