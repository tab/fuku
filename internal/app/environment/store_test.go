package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

func Test_Store_Env(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)

	tests := []struct {
		name     string
		before   func()
		id       string
		expected []model.Env
	}{
		{
			name:     "unknown id has no entries",
			before:   func() {},
			id:       "unknown-id",
			expected: nil,
		},
		{
			name: "cached entries are returned",
			before: func() {
				subject.cache["svc-id"] = []model.Env{{Key: "A", Value: "1"}}
			},
			id:       "svc-id",
			expected: []model.Env{{Key: "A", Value: "1"}},
		},
		{
			name: "empty entries read as none",
			before: func() {
				subject.cache["empty-id"] = []model.Env{}
			},
			id:       "empty-id",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.before()

			got := subject.Env(tt.id)

			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_Store_Env_ReturnsCopy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubscriber := NewMockSubscriber(ctrl)
	mockReader := NewMockReader(ctrl)

	subject := NewStore(mockSubscriber, mockReader)
	subject.cache["svc-id"] = []model.Env{{Key: "A", Value: "1"}}

	first := subject.Env("svc-id")
	first[0].Value = "mutated"
	second := subject.Env("svc-id")

	assert.Equal(t, "1", second[0].Value, "Env() result must not alias the cache")
}
