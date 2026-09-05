package app

import (
	"os"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewShutdown(t *testing.T) {
	shutdown := NewShutdown()

	assert.NotNil(t, shutdown)
	assert.Nil(t, shutdown.Signal())
}

func Test_Shutdown_Signal(t *testing.T) {
	tests := []struct {
		name     string
		before   func(s *Shutdown)
		expected os.Signal
	}{
		{
			name:     "Nothing observed",
			before:   func(_ *Shutdown) {},
			expected: nil,
		},
		{
			name: "Interrupt observed",
			before: func(s *Shutdown) {
				s.Observe(syscall.SIGINT)
			},
			expected: syscall.SIGINT,
		},
		{
			name: "Terminate observed",
			before: func(s *Shutdown) {
				s.Observe(syscall.SIGTERM)
			},
			expected: syscall.SIGTERM,
		},
		{
			name: "Nil signal observed",
			before: func(s *Shutdown) {
				s.Observe(nil)
			},
			expected: nil,
		},
		{
			name: "Application claimed the shutdown first",
			before: func(s *Shutdown) {
				s.Self()
				s.Observe(syscall.SIGTERM)
			},
			expected: nil,
		},
		{
			name: "Application claimed the shutdown after the signal",
			before: func(s *Shutdown) {
				s.Observe(syscall.SIGTERM)
				s.Self()
			},
			expected: syscall.SIGTERM,
		},
		{
			name: "Last observed signal wins",
			before: func(s *Shutdown) {
				s.Observe(syscall.SIGINT)
				s.Observe(syscall.SIGTERM)
			},
			expected: syscall.SIGTERM,
		},
		{
			name: "Claimed twice",
			before: func(s *Shutdown) {
				s.Self()
				s.Self()
				s.Observe(syscall.SIGINT)
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdown := NewShutdown()
			tt.before(shutdown)

			assert.Equal(t, tt.expected, shutdown.Signal())
		})
	}
}

func Test_Shutdown_ConcurrentAccess(t *testing.T) {
	shutdown := NewShutdown()

	var wg sync.WaitGroup

	for range 50 {
		wg.Add(3)

		go func() {
			defer wg.Done()

			shutdown.Observe(syscall.SIGTERM)
		}()

		go func() {
			defer wg.Done()

			shutdown.Self()
		}()

		go func() {
			defer wg.Done()

			shutdown.Signal()
		}()
	}

	wg.Wait()
}
