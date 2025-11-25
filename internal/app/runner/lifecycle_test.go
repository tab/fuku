package runner

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/config/logger"
)

func Test_NewLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	lc := NewLifecycle(mockLogger)

	assert.NotNil(t, lc)
	instance, ok := lc.(*lifecycle)
	assert.True(t, ok)
	assert.Equal(t, mockLogger, instance.log)
}

func Test_Configure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	lc := NewLifecycle(mockLogger)

	cmd := exec.Command("echo", "test")
	assert.Nil(t, cmd.SysProcAttr)

	lc.Configure(cmd)

	assert.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setpgid)
}

func Test_Terminate_NilProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)

	lc := NewLifecycle(mockLogger)

	mockProcess := NewMockProcess(ctrl)
	mockCmd := &exec.Cmd{Process: nil}
	mockProcess.EXPECT().Cmd().Return(mockCmd)

	err := lc.Terminate(mockProcess, time.Second)
	assert.NoError(t, err)
}

func Test_Terminate_ProcessExitsGracefully(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	lc := NewLifecycle(mockLogger)

	cmd := exec.Command("sleep", "10")

	err := cmd.Start()
	if err != nil {
		t.Skip("Cannot start test process")
	}

	done := make(chan struct{})

	go func() {
		cmd.Wait()
		close(done)
	}()

	mockProcess := NewMockProcess(ctrl)
	mockProcess.EXPECT().Cmd().Return(cmd).AnyTimes()
	mockProcess.EXPECT().Name().Return("test-service").AnyTimes()
	mockProcess.EXPECT().Done().Return(done).AnyTimes()

	err = lc.Terminate(mockProcess, 5*time.Second)
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Process should have exited")
	}
}

func Test_Terminate_ProcessRequiresForceKill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logger.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info().Return(nil).AnyTimes()
	mockLogger.EXPECT().Warn().Return(nil).AnyTimes()

	lc := NewLifecycle(mockLogger)

	cmd := exec.Command("sh", "-c", "trap '' TERM; sleep 10")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Start()
	if err != nil {
		t.Skip("Cannot start test process")
	}

	done := make(chan struct{})

	go func() {
		cmd.Wait()
		close(done)
	}()

	mockProcess := NewMockProcess(ctrl)
	mockProcess.EXPECT().Cmd().Return(cmd).AnyTimes()
	mockProcess.EXPECT().Name().Return("test-service").AnyTimes()
	mockProcess.EXPECT().Done().Return(done).AnyTimes()

	err = lc.Terminate(mockProcess, 100*time.Millisecond)
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Process should have been killed")
	}
}
