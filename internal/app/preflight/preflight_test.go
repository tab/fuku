package preflight

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/bus"
	"fuku/internal/config/logger"
)

func Test_NewPreflight(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := logger.NewMockLogger(ctrl)
	mockLog.EXPECT().WithComponent("PREFLIGHT").Return(mockLog)

	p := NewPreflight(bus.NoOp(), mockLog)

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
		processes       []processInfo
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
			processes: []processInfo{
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
			processes: []processInfo{
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
			processes: []processInfo{
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
			processes: []processInfo{
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
			processes: []processInfo{
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

			scan := func() ([]processInfo, error) {
				if tt.scanErr != nil {
					return nil, tt.scanErr
				}

				return tt.processes, nil
			}

			kill := func(pid int32) error {
				killCount++

				return tt.killErr
			}

			p := &preflight{bus: bus.NoOp(), log: mockLog, scan: scan, kill: kill}

			results, err := p.Cleanup(tt.dirs)

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

	scan := func() ([]processInfo, error) {
		return []processInfo{
			{pid: 100, dir: "/project/api", name: "node"},
		}, nil
	}

	kill := func(pid int32) error {
		return nil
	}

	p := &preflight{bus: bus.NoOp(), log: mockLog, scan: scan, kill: kill}

	results, err := p.Cleanup(map[string]string{
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

	scan := func() ([]processInfo, error) {
		return []processInfo{
			{pid: 100, dir: "/project/api-v2", name: "node"},
			{pid: 200, dir: "/project/api", name: "go"},
		}, nil
	}

	kill := func(pid int32) error {
		return nil
	}

	p := &preflight{bus: bus.NoOp(), log: mockLog, scan: scan, kill: kill}

	results, err := p.Cleanup(map[string]string{
		"api": "/project/api",
	})

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int32(200), results[0].PID)
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
