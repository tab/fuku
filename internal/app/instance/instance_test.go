package instance

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
)

func Test_New(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	identity, err := New()
	require.NoError(t, err)

	assert.NotEmpty(t, identity.ID)
	assert.NotEmpty(t, identity.Project)
	assert.Len(t, identity.Fingerprint, FingerprintLength)
	assert.Equal(t, Fingerprint(identity.Project), identity.Fingerprint)
}

func Test_New_UniquePerInstance(t *testing.T) {
	t.Chdir(t.TempDir())

	first, err := New()
	require.NoError(t, err)

	second, err := New()
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, first.Fingerprint, second.Fingerprint)
}

func Test_New_MissingWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, os.RemoveAll(dir))

	_, err := New()
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToResolveProject))
}

func Test_Fingerprint(t *testing.T) {
	tests := []struct {
		name    string
		project string
		other   string
		equal   bool
	}{
		{name: "same directory", project: "/tmp/project", other: "/tmp/project", equal: true},
		{name: "sibling directory", project: "/tmp/project", other: "/tmp/other", equal: false},
		{name: "trailing separator", project: "/tmp/project", other: "/tmp/project/", equal: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Fingerprint(tt.project)

			assert.Len(t, result, FingerprintLength)
			assert.Equal(t, tt.equal, result == Fingerprint(tt.other))
		})
	}
}

func Test_Fingerprint_DoesNotDiscloseThePath(t *testing.T) {
	assert.NotContains(t, Fingerprint("/Users/someone/projects/fuku"), "someone")
	assert.Regexp(t, "^[0-9a-f]+$", Fingerprint("/Users/someone/projects/fuku"))
}
