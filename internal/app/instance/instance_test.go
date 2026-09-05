package instance

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
)

var hexDigest = regexp.MustCompile(`^[0-9a-f]{16}$`)

func Test_NewInstance(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	identity, err := NewInstance()
	require.NoError(t, err)

	assert.NotEmpty(t, identity.ID)
	assert.Equal(t, resolved, identity.Project)
	assert.Equal(t, Fingerprint(resolved), identity.Fingerprint)
}

func Test_NewInstance_UniqueIDSharedFingerprint(t *testing.T) {
	t.Chdir(t.TempDir())

	first, err := NewInstance()
	require.NoError(t, err)

	second, err := NewInstance()
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, first.Fingerprint, second.Fingerprint)
	assert.Equal(t, first.Project, second.Project)
}

func Test_NewInstance_ResolvesSymlink(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	require.NoError(t, os.Mkdir(project, 0750))

	link := filepath.Join(root, "link")
	if err := os.Symlink(project, link); err != nil {
		t.Skip("platform does not allow creating symlinks:", err)
	}

	resolved, err := filepath.EvalSymlinks(project)
	require.NoError(t, err)

	t.Chdir(link)

	identity, err := NewInstance()
	require.NoError(t, err)

	assert.Equal(t, resolved, identity.Project)
	assert.Equal(t, Fingerprint(resolved), identity.Fingerprint)
}

func Test_NewInstance_RemovedWorkingDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, os.RemoveAll(dir))

	identity, err := NewInstance()

	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrFailedToResolveProject))
	assert.Empty(t, identity.ID)
}

func Test_Fingerprint(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		expected string
	}{
		{
			name:     "absolute path",
			project:  "/Users/dev/projects/shop",
			expected: "e6a140ffa7c75886",
		},
		{
			name:     "root",
			project:  "/",
			expected: "8a5edab282632443",
		},
		{
			name:     "relative path",
			project:  "projects/shop",
			expected: "8788a12079d30545",
		},
		{
			name:     "empty path",
			project:  "",
			expected: "e3b0c44298fc1c14",
		},
		{
			name:     "path with spaces and unicode",
			project:  "/Users/dev/Мои проекты/shop",
			expected: "edc41a0cf1aa552f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest := Fingerprint(tt.project)

			assert.Equal(t, tt.expected, digest)
			assert.Len(t, digest, FingerprintLength)
			assert.Regexp(t, hexDigest, digest)
		})
	}
}

func Test_Fingerprint_DistinguishesPaths(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		equal bool
	}{
		{
			name:  "same path",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/shop",
			equal: true,
		},
		{
			name:  "sibling directory",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/blog",
			equal: false,
		},
		{
			name:  "trailing separator",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/shop/",
			equal: false,
		},
		{
			name:  "nested directory",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/shop/api",
			equal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.equal {
				assert.Equal(t, Fingerprint(tt.left), Fingerprint(tt.right))

				return
			}

			assert.NotEqual(t, Fingerprint(tt.left), Fingerprint(tt.right))
		})
	}
}
