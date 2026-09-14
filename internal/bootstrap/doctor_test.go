package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/cli"
	"fuku/internal/config"
)

func Test_DoctorCLI_Run(t *testing.T) {
	tests := []struct {
		name   string
		cmd    *cli.Options
		expect int
	}{
		{
			name:   "config directory missing",
			cmd:    &cli.Options{Type: cli.CommandDoctor, ConfigFile: "missing/fuku.yaml"},
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
			t.Chdir(t.TempDir())

			result := NewDoctorCLI(tt.cmd).Run()

			assert.Equal(t, tt.expect, result)
		})
	}
}
