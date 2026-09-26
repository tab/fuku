package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/platform/buildinfo"
)

func Test_Version_Run(t *testing.T) {
	var stdout bytes.Buffer

	subject := NewVersion(&stdout)

	exitCode, err := subject.Run(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "Version: "+buildinfo.Version+"\n", stdout.String())
}
