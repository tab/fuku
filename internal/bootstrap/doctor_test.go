package bootstrap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/cli"
	"fuku/internal/config"
)

func Test_DoctorCLI_Run(t *testing.T) {
	tests := []struct {
		name   string
		cmd    *cli.Options
		remove bool
		expect int
	}{
		{
			name:   "config directory missing",
			cmd:    &cli.Options{Type: cli.CommandDoctor, ConfigFile: "missing/fuku.yaml"},
			expect: 1,
		},
		{
			name:   "deleted working directory",
			cmd:    &cli.Options{Type: cli.CommandDoctor, Profile: config.Default, DoctorFormat: cli.DoctorFormatSummary},
			remove: true,
			expect: 1,
		},
		{
			name:   "no config",
			cmd:    &cli.Options{Type: cli.CommandDoctor, Profile: config.Default, DoctorFormat: cli.DoctorFormatSummary},
			expect: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			if tt.remove {
				require.NoError(t, os.RemoveAll(dir))
			}

			result := NewDoctorCLI(tt.cmd).Run()

			assert.Equal(t, tt.expect, result)
		})
	}
}
