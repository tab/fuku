package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Help_Run(t *testing.T) {
	var stdout bytes.Buffer

	subject := NewHelp(&stdout)

	exitCode, err := subject.Run(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, Usage+"\n", stdout.String())
}
