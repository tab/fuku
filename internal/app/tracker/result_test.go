package tracker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewResult(t *testing.T) {
	resultInstance := NewResult("test-service")
	assert.NotNil(t, resultInstance)

	instance, ok := resultInstance.(*ServiceResult)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.Equal(t, "test-service", resultInstance.GetName())
	assert.Equal(t, StatusPending, resultInstance.GetStatus())
	assert.Nil(t, resultInstance.GetError())
}

func Test_GetName(t *testing.T) {
	r := NewResult("test-service")
	assert.Equal(t, "test-service", r.GetName())
}

func Test_GetStatus(t *testing.T) {
	r := NewResult("test-service")
	assert.Equal(t, StatusPending, r.GetStatus())
}

func Test_SetStatus(t *testing.T) {
	r := NewResult("test-service")

	tests := []struct {
		name     string
		status   Status
		expected Status
	}{
		{
			name:     "Set to Starting",
			status:   StatusStarting,
			expected: StatusStarting,
		},
		{
			name:     "Set to Running",
			status:   StatusRunning,
			expected: StatusRunning,
		},
		{
			name:     "Set to Failed",
			status:   StatusFailed,
			expected: StatusFailed,
		},
		{
			name:     "Set to Stopped",
			status:   StatusStopped,
			expected: StatusStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.SetStatus(tt.status)
			assert.Equal(t, tt.expected, r.GetStatus())
		})
	}
}

func Test_GetError(t *testing.T) {
	r := NewResult("test-service")
	assert.Nil(t, r.GetError())

	r.SetError(errors.New("sample error"))
	assert.Equal(t, errors.New("sample error"), r.GetError())
}

func Test_SetError(t *testing.T) {
	r := NewResult("test-service")

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "Set nil error",
			err:      nil,
			expected: nil,
		},
		{
			name:     "Set actual error",
			err:      errors.New("sample error"),
			expected: errors.New("sample error"),
		},
		{
			name:     "Set another error",
			err:      errors.New("another error"),
			expected: errors.New("another error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.SetError(tt.err)
			assert.Equal(t, tt.expected, r.GetError())
		})
	}
}

func Test_ConcurrentAccess(t *testing.T) {
	r := NewResult("test-service")

	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			r.SetStatus(StatusRunning)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			r.SetError(errors.New("concurrent error"))
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, StatusRunning, r.GetStatus())
	assert.NotNil(t, r.GetError())
}
