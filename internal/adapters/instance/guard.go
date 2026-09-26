package instance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// refusalFormat is the message shown when another instance owns the project
const refusalFormat = "Error: %v (%s)\nRun 'fuku logs' to follow it or stop that instance before starting another.\n"

// Guard keeps one run per project with an exclusive lock on the project's lock file
type Guard struct {
	identity model.Instance
	stderr   io.Writer
	lock     *os.File
}

// NewGuard creates the single-instance guard for the current project
func NewGuard(identity model.Instance, stderr io.Writer) *Guard {
	return &Guard{
		identity: identity,
		stderr:   stderr,
	}
}

// Check takes the project lock for the process lifetime, or returns ErrInstanceAlreadyRunning if another run holds it
func (g *Guard) Check(context.Context) error {
	return g.acquire(SocketDir)
}

// acquire takes the project lock inside socketDir, or refuses and names the owner when another run holds it
func (g *Guard) acquire(socketDir string) error {
	path := lockPath(socketDir, g.identity.Fingerprint)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToLockProject, err)
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		file.Close()
		fmt.Fprintf(g.stderr, refusalFormat, contracts.ErrInstanceAlreadyRunning, g.owner(socketDir, path))

		return contracts.ErrInstanceAlreadyRunning
	}

	if err != nil {
		file.Close()

		return fmt.Errorf("%w: %w", ErrFailedToLockProject, err)
	}

	// sync: the guard keeps the file, so no finalizer closes it and the kernel releases the lock only at exit
	g.lock = file

	return nil
}

// owner names where the other instance answers: its socket, or the lock file while its socket is not up yet
func (g *Guard) owner(socketDir, path string) string {
	socketPath := SocketPath(socketDir, g.identity.Fingerprint)

	if ProbeSocket(socketPath) != nil {
		return "lock " + path
	}

	return "socket " + socketPath
}
