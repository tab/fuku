package dotenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   Store
		wantOk bool
	}{
		{
			name:   "plain key value",
			line:   "FOO=bar",
			want:   Store{Key: "FOO", Value: "bar"},
			wantOk: true,
		},
		{
			name:   "value preserves spaces and special chars",
			line:   "URL=http://localhost:8080/path?q=1&r=2",
			want:   Store{Key: "URL", Value: "http://localhost:8080/path?q=1&r=2"},
			wantOk: true,
		},
		{
			name:   "export prefix is stripped",
			line:   "export FOO=bar",
			want:   Store{Key: "FOO", Value: "bar"},
			wantOk: true,
		},
		{
			name:   "leading whitespace is trimmed",
			line:   "   KEY=value",
			want:   Store{Key: "KEY", Value: "value"},
			wantOk: true,
		},
		{
			name:   "empty value is preserved",
			line:   "FLAG=",
			want:   Store{Key: "FLAG", Value: ""},
			wantOk: true,
		},
		{
			name:   "blank line skipped",
			line:   "",
			wantOk: false,
		},
		{
			name:   "comment line skipped",
			line:   "# this is a comment",
			wantOk: false,
		},
		{
			name:   "indented comment skipped",
			line:   "   # comment",
			wantOk: false,
		},
		{
			name:   "line without equals skipped",
			line:   "NOTAPAIR",
			wantOk: false,
		},
		{
			name:   "line with only equals skipped",
			line:   "=value",
			wantOk: false,
		},
		{
			name:   "export with no key skipped",
			line:   "export =value",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLine(tt.line)
			assert.Equal(t, tt.wantOk, ok)

			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_MergeFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		fileOrder []string
		want      []Store
	}{
		{
			name:      "single file preserves declaration order",
			files:     map[string]string{".env": "FOO=bar\nBAZ=qux\n"},
			fileOrder: []string{".env"},
			want: []Store{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name: "later file overrides earlier value, key keeps first-appearance position",
			files: map[string]string{
				".env":             "JWT_SECRET=base\nA=1\n",
				".env.development": "JWT_SECRET=dev\nB=2\n",
			},
			fileOrder: []string{".env", ".env.development"},
			want: []Store{
				{Key: "JWT_SECRET", Value: "dev"},
				{Key: "A", Value: "1"},
				{Key: "B", Value: "2"},
			},
		},
		{
			name: "custom load order: .env.development then .env",
			files: map[string]string{
				".env":             "JWT_SECRET=overridden\nADMIN_EMAIL=tab@hub\n",
				".env.development": "JWT_SECRET=dev\nAPP_NAME=hub-api\n",
			},
			fileOrder: []string{".env.development", ".env"},
			want: []Store{
				{Key: "JWT_SECRET", Value: "overridden"},
				{Key: "APP_NAME", Value: "hub-api"},
				{Key: "ADMIN_EMAIL", Value: "tab@hub"},
			},
		},
		{
			name: "missing file is skipped silently",
			files: map[string]string{
				".env": "FOO=bar\n",
			},
			fileOrder: []string{".env", ".env.development", ".env.development.local"},
			want: []Store{
				{Key: "FOO", Value: "bar"},
			},
		},
		{
			name:      "no files in order returns nil",
			files:     map[string]string{".env": "FOO=bar\n"},
			fileOrder: nil,
			want:      nil,
		},
		{
			name: "directory matching path is skipped",
			files: map[string]string{
				".env/keep": "ignored",
			},
			fileOrder: []string{".env"},
			want:      nil,
		},
		{
			name: "parent traversal entry is rejected",
			files: map[string]string{
				".env": "INSIDE=ok\n",
			},
			fileOrder: []string{"../escape", ".env"},
			want: []Store{
				{Key: "INSIDE", Value: "ok"},
			},
		},
		{
			name: "absolute path entry is rejected",
			files: map[string]string{
				".env": "INSIDE=ok\n",
			},
			fileOrder: []string{"/etc/passwd", ".env"},
			want: []Store{
				{Key: "INSIDE", Value: "ok"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			for name, contents := range tt.files {
				full := filepath.Join(dir, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
				require.NoError(t, os.WriteFile(full, []byte(contents), 0o600))
			}

			got := mergeFiles(dir, tt.fileOrder)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_MergeFiles_EmptyInputs(t *testing.T) {
	assert.Nil(t, mergeFiles("", []string{".env"}))
	assert.Nil(t, mergeFiles(t.TempDir(), nil))
}

func Test_IsSafeRelativePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "plain .env name",
			path: ".env",
			want: true,
		},
		{
			name: "nested relative path",
			path: "config/.env",
			want: true,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
		{
			name: "absolute path",
			path: "/etc/passwd",
			want: false,
		},
		{
			name: "parent traversal at start",
			path: "../escape",
			want: false,
		},
		{
			name: "parent traversal segment",
			path: "config/../../escape",
			want: false,
		},
		{
			name: "literal parent",
			path: "..",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSafeRelativePath(tt.path))
		})
	}
}
