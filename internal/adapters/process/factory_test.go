package process

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

func Test_NewFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, mockSink, log)

	assert.NotNil(t, factory)
	assert.Equal(t, tracker, factory.tracker)
	assert.Equal(t, mockSink, factory.sink)
	assert.Equal(t, log, factory.log)
}

func Test_Factory_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, mockSink, log)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "sleep 60", Directory: t.TempDir()}

	proc, err := factory.Start(svc)

	require.NoError(t, err)
	assert.Equal(t, svc, proc.Service())
	assert.Positive(t, proc.PID())

	tracked, exists := tracker.Get(svc.ID)
	assert.True(t, exists)
	assert.Equal(t, proc, tracked)

	require.NoError(t, proc.Terminate())

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the terminated child did not close done")
	}

	assert.True(t, tracker.Untrack(svc.ID, proc))
}

func Test_Factory_Start_MissingDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, mockSink, log)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "sleep 60", Directory: "/nonexistent/directory/path"}

	proc, err := factory.Start(svc)

	require.ErrorIs(t, err, contracts.ErrServiceDirectoryNotExist)
	assert.Nil(t, proc)

	_, exists := tracker.Get(svc.ID)
	assert.False(t, exists)
}

func Test_Factory_Start_DirectoryIsAFile(t *testing.T) {
	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, nil, log)

	file := filepath.Join(t.TempDir(), "api")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "sleep 60", Directory: file}

	proc, err := factory.Start(svc)

	require.ErrorIs(t, err, contracts.ErrFailedToStartCommand)
	assert.Nil(t, proc)

	_, exists := tracker.Get(svc.ID)
	assert.False(t, exists)
}

func Test_Factory_Start_ExitClosesDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, mockSink, log)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "exit 3", Directory: t.TempDir()}

	proc, err := factory.Start(svc)

	require.NoError(t, err)

	select {
	case <-proc.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the child's exit did not close done")
	}

	assert.True(t, tracker.Untrack(svc.ID, proc))
	require.NoError(t, proc.Terminate())
}

func Test_Factory_Start_ReadsTheStreamsToTheirEnd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)
	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()

	factory := NewFactory(tracker, mockSink, mockLog)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "seq 1 50000", Directory: t.TempDir(), LogOutput: []string{"stdout", "stderr"}}

	var last string

	record := func(_, message string) {
		last = message
	}

	mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockSink.EXPECT().Broadcast(svc.Name, gomock.Any()).Do(record).AnyTimes()

	proc, err := factory.Start(svc)

	require.NoError(t, err)

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the child's exit did not close done")
	}

	assert.Equal(t, "50000", last)
}

func Test_Factory_Start_LongLineNeverStallsTheChild(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, mockSink, log)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "head -c 5242880 /dev/zero | tr '\\0' x; echo", Directory: t.TempDir(), LogOutput: []string{"stdout"}}

	var broadcast int

	record := func(_, message string) {
		broadcast = len(message)
	}

	mockSink.EXPECT().Broadcast(svc.Name, gomock.Any()).Do(record)

	proc, err := factory.Start(svc)

	require.NoError(t, err)

	killGroup := func() {
		syscall.Kill(-proc.PID(), syscall.SIGKILL)
	}

	defer killGroup()

	select {
	case <-proc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a 5 MiB line nobody reads stalled the child")
	}

	assert.Equal(t, MaxLineSize, broadcast)
}

func Test_Factory_Start_StdoutCopy(t *testing.T) {
	tracker := NewTracker()
	log := slog.New(slog.DiscardHandler)

	factory := NewFactory(tracker, nil, log)

	logProbe := &model.Readiness{Type: model.ReadinessLog, Pattern: "ready"}
	httpProbe := &model.Readiness{Type: model.ReadinessHTTP, URL: "http://localhost:8080/health"}

	tests := []struct {
		name     string
		service  model.Service
		expected string
		err      error
	}{
		{
			name:     "a service with a log probe gets a copy of its output",
			service:  model.Service{ID: "test-id-api", Name: "api", Command: "echo ready", Directory: t.TempDir(), Readiness: logProbe},
			expected: "ready\n",
		},
		{
			name:    "a service without a log probe gets its copy closed from the start",
			service: model.Service{ID: "test-id-api", Name: "api", Command: "echo ready", Directory: t.TempDir(), Readiness: httpProbe},
			err:     io.ErrClosedPipe,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc, err := factory.Start(tt.service)

			require.NoError(t, err)

			output, err := io.ReadAll(proc.Stdout())

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.expected, string(output))

			select {
			case <-proc.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("the child's exit did not close done")
			}
		})
	}
}

func Test_Factory_Start_DescendantHoldingTheStreams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)
	mockSink := NewMockLogSink(ctrl)

	tracker := NewTracker()

	factory := NewFactory(tracker, mockSink, mockLog)

	svc := model.Service{ID: "test-id-api", Name: "api", Command: "sleep 30 & exit 0", Directory: t.TempDir()}

	mockLog.EXPECT().Info(gomock.Any())

	proc, err := factory.Start(svc)

	require.NoError(t, err)

	killGroup := func() {
		syscall.Kill(-proc.PID(), syscall.SIGKILL)
	}

	defer killGroup()

	select {
	case <-proc.Done():
	case <-time.After(ShutdownTimeout + time.Second):
		t.Fatal("a descendant holding the streams kept done open")
	}
}
