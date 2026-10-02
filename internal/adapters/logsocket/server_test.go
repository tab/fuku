package logsocket

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/adapters/instance"
	"fuku/internal/model"
)

// testProfile is the profile every test server serves
const testProfile = "test"

// readFrom stands in for Registry.Read and runs the callback on the fixture snapshot
func readFrom(snapshot *model.Snapshot) func(func(*model.Snapshot)) {
	return func(fn func(*model.Snapshot)) {
		fn(snapshot)
	}
}

// testIdentity builds the identity of a project of its own, so each test binds its own socket
func testIdentity(t *testing.T) model.Instance {
	t.Helper()

	project := "/Users/dev/projects/" + t.Name()

	return model.Instance{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     project,
		Fingerprint: instance.Fingerprint(project),
	}
}

// testSocketDir creates a socket directory of the test's own, short enough for a unix socket path
func testSocketDir(t *testing.T) string {
	t.Helper()

	//nolint:usetesting // socket path length exceeds macOS limit with t.TempDir
	dir, err := os.MkdirTemp("/tmp", "fuku-test-")
	require.NoError(t, err)

	t.Cleanup(func() { os.RemoveAll(dir) })

	return dir
}

func Test_NewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHub := NewMockHub(ctrl)
	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	identity := model.Instance{
		ID:          "1f0c6e4a-2b8d-4c3e-9a7f-5d6b8c0e1a24",
		Project:     "/Users/dev/projects/shop",
		Fingerprint: instance.Fingerprint("/Users/dev/projects/shop"),
	}

	s := NewServer(mockHub, mockRegistry, identity, log)

	require.NotNil(t, s)
	assert.Equal(t, mockHub, s.hub)
	assert.Equal(t, mockRegistry, s.registry)
	assert.Equal(t, identity.ID, s.instanceID)
	assert.Equal(t, identity.Fingerprint, s.fingerprint)
	assert.Equal(t, instance.SocketPath(instance.SocketDir, identity.Fingerprint), s.socketPath)
}

func Test_Server_run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	api := &model.Service{ID: "api-id", Name: "api"}
	web := &model.Service{ID: "web-id", Name: "web"}
	snapshot := &model.Snapshot{
		Profile:  testProfile,
		Tiers:    []*model.Tier{{Name: "platform", Services: []*model.Service{api, web}}},
		Services: map[string]*model.Service{"api-id": api, "web-id": web},
	}

	resolved := func(context.Context) {}

	cancelled := func(ctx context.Context) {
		<-ctx.Done()
	}

	dir := testSocketDir(t)

	tests := []struct {
		name     string
		before   func(cancel context.CancelFunc)
		profile  string
		services []string
		running  bool
		bound    error
	}{
		{
			name: "binds the socket once the profile is resolved",
			before: func(context.CancelFunc) {
				mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(resolved)
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))
			},
			profile:  testProfile,
			services: []string{"api", "web"},
			running:  true,
		},
		{
			name: "returns without binding when cancelled first",
			before: func(cancel context.CancelFunc) {
				mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(cancelled)
				cancel()
			},
			bound: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			tt.before(cancel)

			s := NewServer(nil, mockRegistry, testIdentity(t), log)
			s.socketPath = instance.SocketPath(dir, s.fingerprint)

			s.run(ctx)
			defer s.close()

			assert.Equal(t, tt.profile, s.profile)
			assert.Equal(t, tt.services, s.services)
			assert.Equal(t, tt.running, s.running.Load())
			assert.ErrorIs(t, s.Bound(ctx), tt.bound)
		})
	}
}

func Test_Server_run_StartFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)
	mockLog := NewMockLogger(ctrl)

	identity := model.Instance{Fingerprint: "nonexistent/fingerprint"}
	snapshot := &model.Snapshot{Profile: testProfile}

	resolved := func(context.Context) {}

	s := NewServer(nil, mockRegistry, identity, mockLog)
	s.socketPath = instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(resolved)
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))
	mockLog.EXPECT().Warn("Failed to start logs server, continuing without it", "error", gomock.Any())

	s.run(t.Context())

	assert.Equal(t, testProfile, s.profile)
	assert.False(t, s.running.Load())
	assert.ErrorContains(t, s.Bound(t.Context()), "failed to listen on socket")
}

func Test_Server_run_CleanupFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes a file from a read-only directory")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)
	mockLog := NewMockLogger(ctrl)

	dir := testSocketDir(t)

	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrUnix{Name: instance.SocketPath(dir, "0123456789abcdef")}))
	require.NoError(t, syscall.Close(fd))
	require.NoError(t, os.Chmod(dir, 0o555))

	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	identity := model.Instance{Fingerprint: instance.Fingerprint("/Users/dev/projects/" + t.Name())}
	snapshot := &model.Snapshot{Profile: testProfile}

	resolved := func(context.Context) {}

	s := NewServer(nil, mockRegistry, identity, mockLog)
	s.socketPath = instance.SocketPath(dir, identity.Fingerprint)

	mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(resolved)
	mockRegistry.EXPECT().Read(gomock.Any()).Do(readFrom(snapshot))
	mockLog.EXPECT().Warn("Socket cleanup failed, continuing startup", "error", gomock.Any())
	mockLog.EXPECT().Warn("Failed to start logs server, continuing without it", "error", gomock.Any())

	s.run(t.Context())

	assert.Equal(t, testProfile, s.profile)
	assert.False(t, s.running.Load())
}

func Test_Server_StartStop(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath

	require.NoError(t, srv.start(t.Context()))
	assert.FileExists(t, socketPath)

	srv.close()

	assert.NoFileExists(t, socketPath)
}

func Test_Server_Start_ActiveSocket_ReturnsError(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	defer listener.Close()

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath

	err = srv.start(t.Context())

	require.ErrorContains(t, err, "socket is already in use")
	assert.False(t, srv.running.Load())
}

func Test_Server_Start_StaleDirectory_ReturnsError(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	require.NoError(t, os.Mkdir(socketPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(socketPath, "child"), []byte("keep"), 0o600))

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath

	err := srv.start(t.Context())

	require.ErrorContains(t, err, "failed to cleanup stale socket")
	assert.False(t, srv.running.Load())
}

func Test_Server_Start_RecoverFromStaleSocket(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	require.NoError(t, os.WriteFile(socketPath, []byte("stale"), 0600))

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath

	err := srv.start(t.Context())
	defer srv.close()

	require.NoError(t, err)

	info, err := os.Lstat(socketPath)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSocket, info.Mode()&os.ModeSocket)
	assert.True(t, srv.running.Load())
}

func Test_Server_Start_ListenError(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := model.Instance{Fingerprint: "nonexistent/fingerprint"}

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	err := srv.start(t.Context())

	require.ErrorContains(t, err, "failed to listen on socket")
	assert.False(t, srv.running.Load())
}

func Test_Server_close_RemoveSocketError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	socketPath := t.TempDir()
	require.NoError(t, os.WriteFile(socketPath+"/child", []byte("keep"), 0600))

	srv := NewServer(nil, nil, model.Instance{}, mockLog)
	srv.socketPath = socketPath
	srv.running.Store(true)

	mockLog.EXPECT().Warn("Failed to remove socket file: "+socketPath, "error", gomock.Any())
	mockLog.EXPECT().Debug("Server stopped")

	srv.close()

	assert.DirExists(t, socketPath)
	assert.False(t, srv.running.Load())
}

// errAccept is the failure the fake listener reports
var errAccept = errors.New("accept: too many open files")

// failingListener fails Accept the given number of times, then reports the listener closed
type failingListener struct {
	net.Listener
	failures int
	accepts  int
}

func (l *failingListener) Accept() (net.Conn, error) {
	l.accepts++
	if l.accepts <= l.failures {
		return nil, errAccept
	}

	return nil, net.ErrClosed
}

func Test_Server_acceptConnections(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	tests := []struct {
		name     string
		before   func(cancel context.CancelFunc)
		failures int
		accepts  int
		elapsed  time.Duration
	}{
		{
			name:    "a closed listener ends the loop",
			before:  func(context.CancelFunc) {},
			accepts: 1,
		},
		{
			name: "a failed accept pauses, doubling from 5ms",
			before: func(context.CancelFunc) {
				mockLog.EXPECT().Error(gomock.Any(), "error", errAccept).Times(3)
			},
			failures: 3,
			accepts:  4,
			elapsed:  35 * time.Millisecond,
		},
		{
			name: "the pause is capped at one second",
			before: func(context.CancelFunc) {
				mockLog.EXPECT().Error(gomock.Any(), "error", errAccept).Times(10)
			},
			failures: 10,
			accepts:  11,
			elapsed:  3275 * time.Millisecond,
		},
		{
			name: "a stop during the pause ends the loop",
			before: func(cancel context.CancelFunc) {
				mockLog.EXPECT().Error(gomock.Any(), "error", errAccept)
				cancel()
			},
			failures: 1,
			accepts:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				listener := &failingListener{failures: tt.failures}
				srv := &Server{listener: listener, log: mockLog}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				tt.before(cancel)

				start := time.Now()

				srv.acceptConnections(ctx)

				assert.Equal(t, tt.accepts, listener.accepts)
				assert.Equal(t, tt.elapsed, time.Since(start))
			})
		})
	}
}

func Test_Server_Start_Producer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	api := &model.Service{ID: "api-id", Name: "api"}
	snapshot := &model.Snapshot{
		Profile:  testProfile,
		Tiers:    []*model.Tier{{Name: "platform", Services: []*model.Service{api}}},
		Services: map[string]*model.Service{"api-id": api},
	}

	resolved := func(context.Context) {}

	waiting := func(ctx context.Context) {
		<-ctx.Done()
	}

	dir := testSocketDir(t)

	tests := []struct {
		name   string
		before func() *Server
	}{
		{
			name: "stop ends a start still waiting for the profile",
			before: func() *Server {
				s := NewServer(nil, mockRegistry, testIdentity(t), log)
				s.socketPath = instance.SocketPath(dir, s.fingerprint)

				mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(waiting)

				require.NoError(t, s.Start(t.Context()))

				return s
			},
		},
		{
			name: "stop closes a bound server and removes its socket",
			before: func() *Server {
				s := NewServer(nil, mockRegistry, testIdentity(t), log)
				s.socketPath = instance.SocketPath(dir, s.fingerprint)
				bound := make(chan struct{})
				readThenSignal := func(fn func(*model.Snapshot)) {
					readFrom(snapshot)(fn)
					close(bound)
				}

				mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(resolved)
				mockRegistry.EXPECT().Read(gomock.Any()).Do(readThenSignal)

				require.NoError(t, s.Start(t.Context()))

				<-bound

				return s
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.before()

			err := s.Stop(t.Context())

			require.NoError(t, err)
			assert.False(t, s.running.Load())
			assert.NoFileExists(t, s.socketPath)
		})
	}
}

func Test_Server_Stop_ContextDone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := NewMockRegistry(ctrl)

	log := slog.New(slog.DiscardHandler)

	release := make(chan struct{})
	held := func(context.Context) {
		<-release
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	s := NewServer(nil, mockRegistry, testIdentity(t), log)

	mockRegistry.EXPECT().WaitResolved(gomock.Any()).Do(held)

	require.NoError(t, s.Start(t.Context()))

	err := s.Stop(ctx)

	close(release)
	<-s.done

	require.ErrorIs(t, err, context.Canceled)
}

func Test_Server_Stop_ContextDone_ClosesABoundServer(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	runCtx, halt := context.WithCancel(t.Context())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath
	srv.halt = halt

	require.NoError(t, srv.start(runCtx))

	err := srv.Stop(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, srv.running.Load())
	assert.NoFileExists(t, socketPath)
}

func Test_Server_close_DisconnectsAClientThatNeverSubscribed(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	identity := testIdentity(t)
	socketPath := instance.SocketPath(testSocketDir(t), identity.Fingerprint)

	srv := NewServer(nil, nil, identity, log)
	srv.socketPath = socketPath

	ctx, cancel := context.WithCancel(t.Context())

	require.NoError(t, srv.start(ctx))

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)

	defer conn.Close()

	closed := make(chan struct{})

	cancel()

	go func() {
		defer close(closed)

		srv.close()
	}()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close waited on a client that never subscribed")
	}

	assert.False(t, srv.running.Load())
}
