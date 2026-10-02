package detach

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// exitInterrupted is the exit code of a detached start the caller aborted, as a shell reports SIGINT
const exitInterrupted = 130

// errExitedEarly reports a child that closed the pipe without a word, such as one killed from outside
var errExitedEarly = errors.New("the detached instance exited before every service was running")

// Starter launches the detached child
type Starter interface {
	Launch() (Process, error)
}

// View renders the startup records while the parent waits
type View interface {
	Open()
	Show(record Record)
	Close()
}

// Command is the parent of a detached run: it launches the child, renders its startup and returns once it settled
type Command struct {
	starter Starter
	view    View
	stdout  io.Writer
	stderr  io.Writer
}

// NewCommand creates the detached run command
func NewCommand(starter Starter, view View, stdout, stderr io.Writer) *Command {
	return &Command{starter: starter, view: view, stdout: stdout, stderr: stderr}
}

// Run waits for the child to report success or failure, and stops it when ctx ends or a signal arrives first
func (c *Command) Run(ctx context.Context) (int, error) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(interrupt)

	child, err := c.starter.Launch()
	if err != nil {
		return 1, err
	}

	c.view.Open()

	lines := make(chan []byte)
	go scan(child.Output(), lines)

	var (
		texts   []string
		running *Record
	)

	for {
		select {
		case <-ctx.Done():
			return c.abort(child, lines)
		case <-interrupt:
			return c.abort(child, lines)
		case line, ok := <-lines:
			if !ok && running != nil {
				ignoreLateSignals()
			}

			if !ok && aborted(ctx, interrupt) {
				return c.abort(child, lines)
			}

			if !ok {
				return c.settle(child, running, texts)
			}

			record, isRecord := decode(line)
			if !isRecord {
				texts = append(texts, string(line))
				running = nil

				continue
			}

			if record.Kind == KindRunning {
				running = &record
			}

			c.view.Show(record)
		}
	}
}

// settle ends the wait once the pipe closed: the child either reported success or exited
func (c *Command) settle(child Process, running *Record, texts []string) (int, error) {
	if running != nil {
		c.view.Close()

		if err := child.Release(); err != nil {
			return 1, fmt.Errorf("failed to release the detached instance: %w", err)
		}

		c.summary(*running)

		return 0, nil
	}

	//nolint:errcheck // a child that closed the pipe is exiting; a read that failed leaves one to stop
	child.Terminate()
	//nolint:errcheck // the exit status adds nothing to the reason the child printed
	child.Wait()

	c.view.Close()

	if len(texts) == 0 {
		return 1, errExitedEarly
	}

	for _, text := range texts {
		fmt.Fprintln(c.stderr, text)
	}

	return 1, nil
}

// abort stops the child and waits for it, draining the pipe so its shutdown never blocks on a write
func (c *Command) abort(child Process, lines <-chan []byte) (int, error) {
	if err := child.Terminate(); err != nil {
		c.view.Close()

		return 1, fmt.Errorf("failed to stop the detached instance: %w", err)
	}

	drain(lines)

	//nolint:errcheck // the child was asked to stop; how it exited changes nothing
	child.Wait()

	c.view.Close()

	return exitInterrupted, nil
}

// summary prints where the running instance can be reached
func (c *Command) summary(running Record) {
	fmt.Fprintf(c.stdout, "Running detached · pid %d · %d services · %s\n", running.PID, running.Count, seconds(running.Duration))

	if running.Address != "" {
		fmt.Fprintf(c.stdout, "API %s\n", running.Address)
	}
}

// ignoreLateSignals drops every later signal, so a released child is never reported as interrupted
func ignoreLateSignals() {
	signal.Ignore(os.Interrupt, syscall.SIGTERM)
}

// aborted reports a wait that ctx or a signal ended, without blocking
func aborted(ctx context.Context, interrupt <-chan os.Signal) bool {
	select {
	case <-ctx.Done():
		return true
	case <-interrupt:
		return true
	default:
		return false
	}
}

// drain discards the rest of the pipe, so the child's shutdown never blocks on a write
func drain(lines <-chan []byte) {
	for range lines {
		continue
	}
}

// scan sends every line of the pipe, of any length, and closes the channel at its end or at a read failure
func scan(output io.Reader, lines chan<- []byte) {
	defer close(lines)

	reader := bufio.NewReader(output)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lines <- bytes.TrimSuffix(line, []byte("\n"))
		}

		if err != nil {
			return
		}
	}
}
