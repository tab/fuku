package preflight

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/app/worker"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

func Test_NewPreflight(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.DefaultConfig()
	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("PREFLIGHT").Return(mockLog)

	p := NewPreflight(cfg, bus.NoOp(), mockLog)

	assert.NotNil(t, p)
}

func Test_Cleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()

	tests := []struct {
		name            string
		dirs            map[string]string
		processes       []entry
		scanErr         error
		killErr         error
		expectedResults int
		expectedKills   int
	}{
		{
			name:            "Empty dirs",
			dirs:            map[string]string{},
			expectedResults: 0,
			expectedKills:   0,
		},
		{
			name: "No matching processes",
			dirs: map[string]string{
				"api": "/project/api",
			},
			processes: []entry{
				{pid: 100, dir: "/other/dir", name: "node"},
			},
			expectedResults: 0,
			expectedKills:   0,
		},
		{
			name: "Matching process killed",
			dirs: map[string]string{
				"api": "/project/api",
			},
			processes: []entry{
				{pid: 100, dir: "/project/api", name: "node"},
			},
			expectedResults: 1,
			expectedKills:   1,
		},
		{
			name: "Multiple matching processes",
			dirs: map[string]string{
				"api": "/project/api",
				"web": "/project/web",
			},
			processes: []entry{
				{pid: 100, dir: "/project/api", name: "node"},
				{pid: 200, dir: "/project/web", name: "go"},
				{pid: 300, dir: "/other/dir", name: "vim"},
			},
			expectedResults: 2,
			expectedKills:   2,
		},
		{
			name: "Skips own PID",
			dirs: map[string]string{
				"api": "/project/api",
			},
			processes: []entry{
				{pid: int32(os.Getpid()), dir: "/project/api", name: "fuku"},
				{pid: 200, dir: "/project/api", name: "node"},
			},
			expectedResults: 1,
			expectedKills:   1,
		},
		{
			name: "Scan failure returns nil",
			dirs: map[string]string{
				"api": "/project/api",
			},
			scanErr:         fmt.Errorf("permission denied"),
			expectedResults: 0,
			expectedKills:   0,
		},
		{
			name: "Kill failure continues to next process",
			dirs: map[string]string{
				"api": "/project/api",
				"web": "/project/web",
			},
			processes: []entry{
				{pid: 100, dir: "/project/api", name: "node"},
				{pid: 200, dir: "/project/web", name: "go"},
			},
			killErr:         fmt.Errorf("operation not permitted"),
			expectedResults: 2,
			expectedKills:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			killCount := 0

			scan := func() ([]entry, error) {
				if tt.scanErr != nil {
					return nil, tt.scanErr
				}

				return tt.processes, nil
			}

			kill := func(pid int32) error {
				killCount++

				return tt.killErr
			}

			cfg := config.DefaultConfig()

			p := &preflight{
				bus:  bus.NoOp(),
				log:  mockLog,
				scan: scan,
				kill: kill,
				pool: worker.NewWorkerPool(cfg),
			}

			results, err := p.Cleanup(context.Background(), tt.dirs)

			assert.NoError(t, err)
			assert.Len(t, results, tt.expectedResults)
			assert.Equal(t, tt.expectedKills, killCount)
		})
	}
}

func Test_Cleanup_ResultFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	scan := func() ([]entry, error) {
		return []entry{
			{pid: 100, dir: "/project/api", name: "node"},
		}, nil
	}

	kill := func(pid int32) error {
		return nil
	}

	cfg := config.DefaultConfig()

	p := &preflight{
		bus:  bus.NoOp(),
		log:  mockLog,
		scan: scan,
		kill: kill,
		pool: worker.NewWorkerPool(cfg),
	}

	results, err := p.Cleanup(context.Background(), map[string]string{
		"api": "/project/api",
	})

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "api", results[0].Service)
	assert.Equal(t, int32(100), results[0].PID)
	assert.Equal(t, "node", results[0].Name)
}

func Test_Cleanup_MatchesExactDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()

	scan := func() ([]entry, error) {
		return []entry{
			{pid: 100, dir: "/project/api-v2", name: "node"},
			{pid: 200, dir: "/project/api", name: "go"},
		}, nil
	}

	kill := func(pid int32) error {
		return nil
	}

	cfg := config.DefaultConfig()

	p := &preflight{
		bus:  bus.NoOp(),
		log:  mockLog,
		scan: scan,
		kill: kill,
		pool: worker.NewWorkerPool(cfg),
	}

	results, err := p.Cleanup(context.Background(), map[string]string{
		"api": "/project/api",
	})

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int32(200), results[0].PID)
}

func Test_Cleanup_ContextCancellationStopsKills(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().Info().Return(nil).AnyTimes()
	mockLog.EXPECT().Warn().Return(nil).AnyTimes()

	scan := func() ([]entry, error) {
		return []entry{
			{pid: 100, dir: "/project/api", name: "node"},
			{pid: 200, dir: "/project/web", name: "go"},
			{pid: 300, dir: "/project/db", name: "postgres"},
		}, nil
	}

	killCount := 0
	kill := func(pid int32) error {
		killCount++

		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := config.DefaultConfig()
	cfg.Concurrency.Workers = 1

	p := &preflight{
		bus:  bus.NoOp(),
		log:  mockLog,
		scan: scan,
		kill: kill,
		pool: worker.NewWorkerPool(cfg),
	}

	results, err := p.Cleanup(ctx, map[string]string{
		"api": "/project/api",
		"web": "/project/web",
		"db":  "/project/db",
	})

	assert.NoError(t, err)
	assert.Less(t, len(results), 3)
	assert.Equal(t, killCount, len(results))
}

func Test_Scan(t *testing.T) {
	entries, err := scan()

	assert.NoError(t, err)
	assert.NotEmpty(t, entries)

	ownPID := int32(os.Getpid())
	found := false

	for _, e := range entries {
		if e.pid == ownPID {
			found = true

			break
		}
	}

	assert.True(t, found)
}

func Test_Kill_ProcessExitsOnSIGTERM(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())

	pid := int32(cmd.Process.Pid) // #nosec G115 -- PID fits in int32

	// Reap the child when it exits so kill() can detect termination via signal 0
	go cmd.Wait() //nolint:errcheck

	err := kill(pid)

	assert.NoError(t, err)
}

func Test_Kill_ProcessNotFound(t *testing.T) {
	err := kill(2147483647)

	assert.NoError(t, err)
}

func Test_SortedKeys(t *testing.T) {
	m := map[string]string{
		"charlie": "c",
		"alpha":   "a",
		"bravo":   "b",
	}

	keys := sortedKeys(m)

	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, keys)
}
