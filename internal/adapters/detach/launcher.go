package detach

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"fuku/internal/adapters/cli"
)

// Process is the running detached child as its parent sees it
type Process interface {
	Output() io.Reader
	Terminate() error
	Wait() error
	Release() error
}

// Launcher starts the detached child: the same binary in its own session, its standard error piped to the parent
type Launcher struct {
	options Options
}

// NewLauncher creates the launcher of the detached child
func NewLauncher(options Options) *Launcher {
	return &Launcher{options: options}
}

// Launch starts the child as `run <profile> --no-ui` with the child marker and the config the parent loaded
func (l *Launcher) Launch() (Process, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to locate the fuku binary: %w", err)
	}

	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", os.DevNull, err)
	}

	defer null.Close()

	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create the startup pipe: %w", err)
	}

	defer writer.Close()

	cmd := exec.Command(executable, l.args()...)
	cmd.Stdin = null
	cmd.Stdout = null
	cmd.Stderr = writer
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		reader.Close()

		return nil, fmt.Errorf("failed to start the detached instance: %w", err)
	}

	return &Child{cmd: cmd, output: reader}, nil
}

// args are the arguments of the child command
func (l *Launcher) args() []string {
	args := []string{
		cli.CommandRun.String(), l.options.Profile,
		"--" + cli.FlagNoUI.String(),
		"--" + cli.FlagDetachedChild.String(),
	}

	if l.options.ConfigFile != "" {
		args = append(args, "--"+cli.FlagConfig.String(), l.options.ConfigFile)
	}

	return args
}

// Child is a started detached child
type Child struct {
	cmd    *exec.Cmd
	output *os.File
}

// Output is the read end of the startup pipe
func (c *Child) Output() io.Reader {
	return c.output
}

// Terminate asks the child to stop gracefully
func (c *Child) Terminate() error {
	return c.cmd.Process.Signal(syscall.SIGTERM)
}

// Wait waits for the child to exit and closes the pipe
func (c *Child) Wait() error {
	defer c.output.Close()

	return c.cmd.Wait()
}

// Release closes the pipe and lets the child run on after the parent exits
func (c *Child) Release() error {
	c.output.Close()

	return c.cmd.Process.Release()
}
