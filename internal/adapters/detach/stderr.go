package detach

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Stderr is the standard error of the detached child, the pipe its parent reads until the start succeeds
type Stderr struct {
	file *os.File
}

// NewStderr creates the handle on the process standard error
func NewStderr() *Stderr {
	return &Stderr{file: os.Stderr}
}

// Write writes to the standard error
func (s *Stderr) Write(p []byte) (int, error) {
	return s.file.Write(p)
}

// Release points the standard error at the null device, which closes the pipe to the parent
func (s *Stderr) Release() error {
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", os.DevNull, err)
	}

	defer null.Close()

	if err := unix.Dup2(int(null.Fd()), int(s.file.Fd())); err != nil {
		return fmt.Errorf("failed to release the standard error: %w", err)
	}

	return nil
}
