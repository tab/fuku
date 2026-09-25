package process

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"fuku/internal/contracts"
)

// launch is a command prepared to start
type launch struct {
	cmd *exec.Cmd
	dir string
}

// prepare builds the shell command for a service in its own process group, writing its streams to stdout and stderr
func prepare(command, directory string, stdout, stderr io.Writer) (*launch, error) {
	dir, err := resolveDir(directory)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = ShutdownTimeout

	return &launch{cmd: cmd, dir: dir}, nil
}

// resolveDir validates a service directory and returns it as an absolute path
func resolveDir(dir string) (string, error) {
	serviceDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %s", contracts.ErrServiceDirectoryNotExist, serviceDir)
	}

	return serviceDir, nil
}
