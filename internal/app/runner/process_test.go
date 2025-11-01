package runner

import (
	"errors"
	"io"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Name(t *testing.T) {
	p := &process{
		name:  "test-service",
		done:  make(chan struct{}),
		ready: make(chan error),
	}

	assert.Equal(t, "test-service", p.Name())
}

func Test_Cmd(t *testing.T) {
	expectedCmd := exec.Command("make", "run")
	p := &process{
		name:  "test-service",
		cmd:   expectedCmd,
		done:  make(chan struct{}),
		ready: make(chan error),
	}

	assert.Equal(t, expectedCmd, p.Cmd())
}

func Test_Done(t *testing.T) {
	p := &process{
		name:  "test-service",
		done:  make(chan struct{}),
		ready: make(chan error),
	}

	doneChan := p.Done()
	assert.NotNil(t, doneChan)

	go func() {
		close(p.done)
	}()

	_, ok := <-doneChan
	assert.False(t, ok, "done channel should be closed")
}

func Test_Ready(t *testing.T) {
	p := &process{
		name:  "test-service",
		done:  make(chan struct{}),
		ready: make(chan error, 1),
	}

	readyChan := p.Ready()
	assert.NotNil(t, readyChan)

	testErr := errors.New("test error")
	go func() {
		p.ready <- testErr
		close(p.ready)
	}()

	err, ok := <-readyChan
	assert.True(t, ok, "should receive error before channel is closed")
	assert.Equal(t, testErr, err)

	_, ok = <-readyChan
	assert.False(t, ok, "ready channel should be closed")
}

func Test_SignalReady(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectErrSent bool
	}{
		{name: "Signal ready with nil error", err: nil, expectErrSent: false},
		{name: "Signal ready with error", err: errors.New("startup failed"), expectErrSent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &process{
				name:  "test-service",
				done:  make(chan struct{}),
				ready: make(chan error, 1),
			}

			p.SignalReady(tt.err)

			if tt.expectErrSent {
				select {
				case err := <-p.Ready():
					assert.Equal(t, tt.err, err)
				default:
					t.Fatal("expected error to be sent to ready channel")
				}
			}

			select {
			case _, ok := <-p.Ready():
				assert.False(t, ok, "ready channel should be closed")
			default:
				t.Fatal("ready channel should be closed")
			}
		})
	}
}

func Test_StdoutReader(t *testing.T) {
	_, writer := io.Pipe()
	defer writer.Close()

	reader, _ := io.Pipe()
	p := &process{
		name:         "test-service",
		done:         make(chan struct{}),
		ready:        make(chan error),
		stdoutReader: reader,
	}

	assert.Equal(t, reader, p.StdoutReader())
}

func Test_StderrReader(t *testing.T) {
	_, writer := io.Pipe()
	defer writer.Close()

	reader, _ := io.Pipe()
	p := &process{
		name:         "test-service",
		done:         make(chan struct{}),
		ready:        make(chan error),
		stderrReader: reader,
	}

	assert.Equal(t, reader, p.StderrReader())
}
