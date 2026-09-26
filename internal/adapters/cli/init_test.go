package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Init_Run(t *testing.T) {
	created := func(stdout io.Writer) (int, error) {
		_, err := io.WriteString(stdout, "Created fuku.yaml\n")

		return 0, err
	}
	unwritable := func(io.Writer) (int, error) {
		return 1, assert.AnError
	}

	tests := []struct {
		name           string
		create         func(stdout io.Writer) (int, error)
		stdout         *bytes.Buffer
		expectedExit   int
		expectedErr    error
		expectedOutput string
	}{
		{
			name:           "writes the template through the injected stdout",
			create:         created,
			stdout:         &bytes.Buffer{},
			expectedExit:   0,
			expectedOutput: "Created fuku.yaml\n",
		},
		{
			name:           "unwritable template exits 1 with the reason",
			create:         unwritable,
			stdout:         &bytes.Buffer{},
			expectedExit:   1,
			expectedErr:    assert.AnError,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, err := NewInit(tt.create, tt.stdout).Run(t.Context())

			require.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, tt.expectedExit, exitCode)
			assert.Equal(t, tt.expectedOutput, tt.stdout.String())
		})
	}
}
