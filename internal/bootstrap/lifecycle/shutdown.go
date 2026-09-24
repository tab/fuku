package lifecycle

import (
	"os"
	"sync"

	"go.uber.org/fx"
)

// Arbiter records the first terminal outcome of the run and derives the exit code from it
type Arbiter struct {
	shutdowner fx.Shutdowner
	mu         sync.Mutex
	decided    bool
	code       int
	cause      error
	sig        os.Signal
}

// NewArbiter creates the outcome arbiter
func NewArbiter(shutdowner fx.Shutdowner) *Arbiter {
	return &Arbiter{shutdowner: shutdowner}
}

// Fail records a runtime failure as the terminal outcome and stops the container with exit code 1
func (a *Arbiter) Fail(err error) {
	a.decide(1, err)
}

// complete records the command's exit code and its error as the terminal outcome and stops the container with the code
func (a *Arbiter) complete(code int, err error) {
	a.decide(code, err)
}

// observe records the stopping OS signal as the terminal outcome with exit code 0, unless one was recorded first
func (a *Arbiter) observe(sig os.Signal) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.decided {
		return
	}

	a.decided = true
	a.sig = sig
}

// signal returns the OS signal that initiated the shutdown (nil when the outcome was decided from inside)
func (a *Arbiter) signal() os.Signal {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.sig
}

// outcome returns the exit code and the failure to report (0 and nil when nothing was recorded)
func (a *Arbiter) outcome() (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.code, a.cause
}

// decide keeps the first outcome and asks the container to stop with its code
func (a *Arbiter) decide(code int, cause error) {
	a.mu.Lock()

	if a.decided {
		a.mu.Unlock()

		return
	}

	a.decided = true
	a.code = code
	a.cause = cause
	a.mu.Unlock()

	//nolint:errcheck // a second shutdown signal is dropped by design
	a.shutdowner.Shutdown(fx.ExitCode(code))
}
