package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewGuard(t *testing.T) {
	g := NewGuard()

	assert.NotNil(t, g)
}

func Test_Guard_Lock_Success(t *testing.T) {
	g := NewGuard()

	result := g.Lock("test-service")

	assert.True(t, result)
	assert.True(t, g.IsLocked("test-service"))
}

func Test_Guard_Lock_AlreadyLocked(t *testing.T) {
	g := NewGuard()

	first := g.Lock("test-service")
	assert.True(t, first)

	second := g.Lock("test-service")
	assert.False(t, second)
	assert.True(t, g.IsLocked("test-service"))
}

func Test_Guard_Unlock(t *testing.T) {
	g := NewGuard()

	g.Lock("test-service")
	assert.True(t, g.IsLocked("test-service"))

	g.Unlock("test-service")
	assert.False(t, g.IsLocked("test-service"))
}

func Test_Guard_Unlock_NotLocked(t *testing.T) {
	g := NewGuard()

	g.Unlock("test-service")
	assert.False(t, g.IsLocked("test-service"))
}

func Test_Guard_IsLocked_DefaultsFalse(t *testing.T) {
	g := NewGuard()

	locked := g.IsLocked("new-service")

	assert.False(t, locked)
}

func Test_Guard_IndependentServices(t *testing.T) {
	g := NewGuard()

	g.Lock("service-a")
	g.Lock("service-b")

	assert.True(t, g.IsLocked("service-a"))
	assert.True(t, g.IsLocked("service-b"))

	g.Unlock("service-a")

	assert.False(t, g.IsLocked("service-a"))
	assert.True(t, g.IsLocked("service-b"))
}

func Test_Guard_CanLockAfterUnlock(t *testing.T) {
	g := NewGuard()

	assert.True(t, g.Lock("test-service"))
	g.Unlock("test-service")

	assert.True(t, g.Lock("test-service"))
	assert.True(t, g.IsLocked("test-service"))
}
