package environment

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Store_merge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)

	tests := []struct {
		name     string
		before   func()
		dir      string
		files    []string
		expected []model.Env
	}{
		{
			name: "single file preserves declaration order",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "FOO", Value: "bar"}, {Key: "BAZ", Value: "qux"}}, nil)
			},
			dir:   "svc",
			files: []string{".env"},
			expected: []model.Env{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name: "later file overrides earlier value, key keeps first-appearance position",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "JWT_SECRET", Value: "base"}, {Key: "A", Value: "1"}}, nil)
				mockReader.EXPECT().Read("svc", ".env.development").Return([]model.Env{{Key: "JWT_SECRET", Value: "dev"}, {Key: "B", Value: "2"}}, nil)
			},
			dir:   "svc",
			files: []string{".env", ".env.development"},
			expected: []model.Env{
				{Key: "JWT_SECRET", Value: "dev"},
				{Key: "A", Value: "1"},
				{Key: "B", Value: "2"},
			},
		},
		{
			name: "custom load order: .env.development then .env",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env.development").Return([]model.Env{{Key: "JWT_SECRET", Value: "dev"}, {Key: "APP_NAME", Value: "hub-api"}}, nil)
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "JWT_SECRET", Value: "overridden"}, {Key: "ADMIN_EMAIL", Value: "tab@hub"}}, nil)
			},
			dir:   "svc",
			files: []string{".env.development", ".env"},
			expected: []model.Env{
				{Key: "JWT_SECRET", Value: "overridden"},
				{Key: "APP_NAME", Value: "hub-api"},
				{Key: "ADMIN_EMAIL", Value: "tab@hub"},
			},
		},
		{
			name: "missing file is skipped silently",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "FOO", Value: "bar"}}, nil)
				mockReader.EXPECT().Read("svc", ".env.development").Return(nil, fs.ErrNotExist)
				mockReader.EXPECT().Read("svc", ".env.development.local").Return(nil, fs.ErrNotExist)
			},
			dir:   "svc",
			files: []string{".env", ".env.development", ".env.development.local"},
			expected: []model.Env{
				{Key: "FOO", Value: "bar"},
			},
		},
		{
			name: "unreadable file is skipped and later files still merge",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return(nil, fs.ErrPermission)
				mockReader.EXPECT().Read("svc", ".env.local").Return([]model.Env{{Key: "INSIDE", Value: "ok"}}, nil)
			},
			dir:   "svc",
			files: []string{".env", ".env.local"},
			expected: []model.Env{
				{Key: "INSIDE", Value: "ok"},
			},
		},
		{
			name: "parse error drops that file only",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return([]model.Env{{Key: "A", Value: "1"}}, nil)
				mockReader.EXPECT().Read("svc", ".env.local").Return(nil, assert.AnError)
				mockReader.EXPECT().Read("svc", ".env.development").Return([]model.Env{{Key: "A", Value: "3"}}, nil)
			},
			dir:   "svc",
			files: []string{".env", ".env.local", ".env.development"},
			expected: []model.Env{
				{Key: "A", Value: "3"},
			},
		},
		{
			name: "every file failing yields no entries",
			before: func() {
				mockReader.EXPECT().Read("svc", ".env").Return(nil, fs.ErrNotExist)
			},
			dir:      "svc",
			files:    []string{".env"},
			expected: nil,
		},
		{
			name:     "no files reads nothing",
			before:   func() {},
			dir:      "svc",
			files:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			got := subject.merge(tt.dir, tt.files)

			assert.Equal(t, tt.expected, got)
		})
	}
}
