package instance

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_NewInstance_UnresolvableWorkingDir(t *testing.T) {
	tests := []struct {
		name   string
		before func(t *testing.T)
	}{
		{
			name: "a removed working directory",
			before: func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)

				require.NoError(t, os.RemoveAll(dir))
			},
		},
		{
			name: "a removed working directory PWD still resolves to",
			before: func(t *testing.T) {
				if runtime.GOOS != "linux" {
					t.Skip("only Linux exposes the working directory under /proc/self/cwd")
				}

				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("PWD", "/proc/self/cwd")

				require.NoError(t, os.RemoveAll(dir))
			},
		},
		{
			name: "a working directory the process cannot search",
			before: func(t *testing.T) {
				if os.Geteuid() == 0 {
					t.Skip("root searches a directory without permission bits")
				}

				dir := t.TempDir()
				t.Chdir(dir)

				require.NoError(t, os.Chmod(dir, 0))
				t.Cleanup(func() { os.Chmod(dir, 0o700) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t)

			identity, err := NewInstance()

			require.ErrorIs(t, err, ErrFailedToResolveProject)
			assert.Empty(t, identity.ID)
		})
	}
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

func Test_Fingerprint_SamePath(t *testing.T) {
	left := Fingerprint("/Users/dev/projects/shop")
	right := Fingerprint("/Users/dev/projects/shop")

	assert.Equal(t, left, right)
}

func Test_Fingerprint_DistinguishesPaths(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{
			name:  "sibling directory",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/blog",
		},
		{
			name:  "trailing separator",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/shop/",
		},
		{
			name:  "nested directory",
			left:  "/Users/dev/projects/shop",
			right: "/Users/dev/projects/shop/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := Fingerprint(tt.left)
			right := Fingerprint(tt.right)

			assert.NotEqual(t, left, right)
		})
	}
}

func Test_SocketPath(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		fingerprint string
		expected    string
	}{
		{
			name:        "default directory",
			dir:         "/tmp",
			fingerprint: "0123456789abcdef",
			expected:    "/tmp/fuku-0123456789abcdef.sock",
		},
		{
			name:        "custom directory",
			dir:         "/var/run",
			fingerprint: "fedcba9876543210",
			expected:    "/var/run/fuku-fedcba9876543210.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SocketPath(tt.dir, tt.fingerprint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_UserConfigPath(t *testing.T) {
	tests := []struct {
		name      string
		before    func(t *testing.T)
		expected  string
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name: "the file inside the fuku directory",
			before: func(t *testing.T) {
				t.Setenv("XDG_CONFIG_HOME", "/home/dev/.config")
				t.Setenv("HOME", "/home/dev")
			},
			expected:  string(filepath.Separator) + filepath.Join("fuku", "telemetry.id"),
			assertErr: assert.NoError,
		},
		{
			name: "no config directory",
			before: func(t *testing.T) {
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "")
			},
			assertErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before(t)

			path, err := UserConfigPath("telemetry.id")

			tt.assertErr(t, err)
			assert.True(t, strings.HasSuffix(path, tt.expected))
		})
	}
}
