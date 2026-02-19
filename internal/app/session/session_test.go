package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/errors"
	"fuku/internal/config/logger"
)

func newTestSession(t *testing.T) *session {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".fuku", "session.json")

	return &session{path: path}
}

func Test_Save_CreatesDirectoryAndFile(t *testing.T) {
	s := newTestSession(t)

	state := &State{
		Profile:   "default",
		StartedAt: time.Now(),
		Entries:   []Entry{{Service: "api", PID: 1234, StartedAt: time.Now()}},
	}

	err := s.Save(state)
	require.NoError(t, err)

	_, err = os.Stat(s.path)
	assert.NoError(t, err)
}

func Test_Save_OverwritesExistingFile(t *testing.T) {
	s := newTestSession(t)

	state1 := &State{Profile: "first", StartedAt: time.Now()}
	require.NoError(t, s.Save(state1))

	state2 := &State{Profile: "second", StartedAt: time.Now()}
	require.NoError(t, s.Save(state2))

	loaded, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, "second", loaded.Profile)
}

func Test_Load_FileNotFound(t *testing.T) {
	s := newTestSession(t)

	state, err := s.Load()

	assert.Nil(t, state)
	assert.ErrorIs(t, err, errors.ErrSessionNotFound)
}

func Test_Load_CorruptedFile(t *testing.T) {
	s := newTestSession(t)

	dir := filepath.Dir(s.path)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(s.path, []byte("not json"), 0644))

	state, err := s.Load()

	assert.Nil(t, state)
	assert.ErrorIs(t, err, errors.ErrSessionCorrupted)
}

func Test_Load_ValidFile(t *testing.T) {
	s := newTestSession(t)

	now := time.Now().Truncate(time.Millisecond)
	original := &State{
		Profile:   "core",
		StartedAt: now,
		Entries: []Entry{
			{Service: "api", PID: 100, StartedAt: now},
			{Service: "db", PID: 200, StartedAt: now},
		},
	}

	require.NoError(t, s.Save(original))

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Equal(t, original.Profile, loaded.Profile)
	assert.Len(t, loaded.Entries, 2)
	assert.Equal(t, "api", loaded.Entries[0].Service)
	assert.Equal(t, 100, loaded.Entries[0].PID)
	assert.Equal(t, "db", loaded.Entries[1].Service)
}

func Test_Delete_FileExists(t *testing.T) {
	s := newTestSession(t)

	require.NoError(t, s.Save(&State{Profile: "test"}))

	err := s.Delete()
	assert.NoError(t, err)

	_, err = os.Stat(s.path)
	assert.True(t, os.IsNotExist(err))
}

func Test_Delete_FileNotExist(t *testing.T) {
	s := newTestSession(t)

	err := s.Delete()
	assert.NoError(t, err)
}

func Test_Add_NoExistingState(t *testing.T) {
	s := newTestSession(t)

	entry := Entry{Service: "api", PID: 1234, StartedAt: time.Now()}

	err := s.Add(entry)
	require.NoError(t, err)

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Len(t, loaded.Entries, 1)
	assert.Equal(t, "api", loaded.Entries[0].Service)
	assert.Equal(t, 1234, loaded.Entries[0].PID)
}

func Test_Add_ExistingState(t *testing.T) {
	s := newTestSession(t)

	require.NoError(t, s.Save(&State{
		Profile:   "core",
		StartedAt: time.Now(),
		Entries:   []Entry{{Service: "api", PID: 100, StartedAt: time.Now()}},
	}))

	err := s.Add(Entry{Service: "db", PID: 200, StartedAt: time.Now()})
	require.NoError(t, err)

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Len(t, loaded.Entries, 2)
	assert.Equal(t, "api", loaded.Entries[0].Service)
	assert.Equal(t, "db", loaded.Entries[1].Service)
}

func Test_Add_ReplacesExistingService(t *testing.T) {
	s := newTestSession(t)

	require.NoError(t, s.Save(&State{
		Profile:   "core",
		StartedAt: time.Now(),
		Entries:   []Entry{{Service: "api", PID: 100, StartedAt: time.Now()}},
	}))

	err := s.Add(Entry{Service: "api", PID: 999, StartedAt: time.Now()})
	require.NoError(t, err)

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Len(t, loaded.Entries, 1)
	assert.Equal(t, "api", loaded.Entries[0].Service)
	assert.Equal(t, 999, loaded.Entries[0].PID)
}

func Test_Remove_ServiceExists(t *testing.T) {
	s := newTestSession(t)

	require.NoError(t, s.Save(&State{
		Profile:   "core",
		StartedAt: time.Now(),
		Entries: []Entry{
			{Service: "api", PID: 100, StartedAt: time.Now()},
			{Service: "db", PID: 200, StartedAt: time.Now()},
		},
	}))

	err := s.Remove("api")
	require.NoError(t, err)

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Len(t, loaded.Entries, 1)
	assert.Equal(t, "db", loaded.Entries[0].Service)
}

func Test_Remove_ServiceNotFound(t *testing.T) {
	s := newTestSession(t)

	require.NoError(t, s.Save(&State{
		Profile:   "core",
		StartedAt: time.Now(),
		Entries:   []Entry{{Service: "api", PID: 100, StartedAt: time.Now()}},
	}))

	err := s.Remove("nonexistent")
	require.NoError(t, err)

	loaded, err := s.Load()
	require.NoError(t, err)

	assert.Len(t, loaded.Entries, 1)
}

func Test_Remove_NoSessionFile(t *testing.T) {
	s := newTestSession(t)

	err := s.Remove("api")
	assert.NoError(t, err)
}

func Test_VerifyPID_ProcessNotRunning(t *testing.T) {
	entry := Entry{
		Service:   "test",
		PID:       999999,
		StartedAt: time.Now(),
	}

	assert.False(t, VerifyPID(entry))
}

func Test_KillOrphans_NoLiveProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Debug().Return(nil).AnyTimes()

	state := &State{
		Profile:   "test",
		StartedAt: time.Now(),
		Entries: []Entry{
			{Service: "api", PID: 999998, StartedAt: time.Now()},
			{Service: "db", PID: 999999, StartedAt: time.Now()},
		},
	}

	killed := KillOrphans(state, mockLog)
	assert.Equal(t, 0, killed)
}

func Test_Load_CorruptedFile_InvalidJSON(t *testing.T) {
	s := newTestSession(t)

	dir := filepath.Dir(s.path)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(s.path, []byte("{broken"), 0644))

	state, err := s.Load()

	assert.Nil(t, state)
	assert.ErrorIs(t, err, errors.ErrSessionCorrupted)
}

func Test_Save_MarshalRoundtrip(t *testing.T) {
	s := newTestSession(t)

	now := time.Now().Truncate(time.Millisecond)
	state := &State{
		Profile:   "full",
		StartedAt: now,
		Entries: []Entry{
			{Service: "api", PID: 100, StartedAt: now},
		},
	}

	require.NoError(t, s.Save(state))

	data, err := os.ReadFile(s.path)
	require.NoError(t, err)

	var loaded State
	require.NoError(t, json.Unmarshal(data, &loaded))

	assert.Equal(t, "full", loaded.Profile)
	assert.Len(t, loaded.Entries, 1)
}
