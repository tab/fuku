package process

import (
	"errors"
	"io"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	cmd := exec.Command("echo", "test")
	stdoutReader, _ := io.Pipe()
	stderrReader, _ := io.Pipe()

	handle := NewProcess(Params{
		Name:         "test-service",
		Cmd:          cmd,
		StdoutReader: stdoutReader,
		StderrReader: stderrReader,
	})

	assert.NotNil(t, handle)
	assert.Equal(t, "test-service", handle.Name())
	assert.Equal(t, cmd, handle.Cmd())
	assert.Equal(t, stdoutReader, handle.StdoutReader())
	assert.Equal(t, stderrReader, handle.StderrReader())
}

func Test_Name(t *testing.T) {
	handle := NewProcess(Params{Name: "test-service"})

	assert.Equal(t, "test-service", handle.Name())
}

func Test_Cmd(t *testing.T) {
	expectedCmd := exec.Command("make", "run")
	handle := NewProcess(Params{
		Name: "test-service",
		Cmd:  expectedCmd,
	})

	assert.Equal(t, expectedCmd, handle.Cmd())
}

func Test_Done(t *testing.T) {
	handle := NewProcess(Params{Name: "test-service"})

	doneChan := handle.Done()
	assert.NotNil(t, doneChan)

	go func() {
		handle.Close()
	}()

	_, ok := <-doneChan
	assert.False(t, ok, "done channel should be closed")
}

func Test_Ready(t *testing.T) {
	handle := NewProcess(Params{Name: "test-service"})

	readyChan := handle.Ready()
	assert.NotNil(t, readyChan)

	testErr := errors.New("test error")

	go func() {
		handle.SignalReady(testErr)
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
		{
			name:          "Signal ready with nil error",
			err:           nil,
			expectErrSent: false,
		},
		{
			name:          "Signal ready with error",
			err:           errors.New("startup failed"),
			expectErrSent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := NewProcess(Params{Name: "test-service"})

			handle.SignalReady(tt.err)

			if tt.expectErrSent {
				select {
				case err := <-handle.Ready():
					assert.Equal(t, tt.err, err)
				default:
					t.Fatal("expected error to be sent to ready channel")
				}
			}

			select {
			case _, ok := <-handle.Ready():
				assert.False(t, ok, "ready channel should be closed")
			default:
				t.Fatal("ready channel should be closed")
			}
		})
	}
}

func Test_StdoutReader(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	handle := NewProcess(Params{
		Name:         "test-service",
		StdoutReader: reader,
	})

	assert.Equal(t, reader, handle.StdoutReader())
}

func Test_StderrReader(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	handle := NewProcess(Params{
		Name:         "test-service",
		StderrReader: reader,
	})

	assert.Equal(t, reader, handle.StderrReader())
}

func Test_Handle_Close(t *testing.T) {
	handle := NewProcess(Params{Name: "test-service"})

	handle.Close()

	select {
	case <-handle.Done():
	default:
		t.Fatal("done channel should be closed after Close()")
	}
}
