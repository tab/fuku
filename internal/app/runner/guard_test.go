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
}

func Test_Guard_Lock_AlreadyLocked(t *testing.T) {
	g := NewGuard()

	first := g.Lock("test-service")
	assert.True(t, first)

	second := g.Lock("test-service")
	assert.False(t, second)
}

func Test_Guard_Unlock_AllowsRelock(t *testing.T) {
	g := NewGuard()

	g.Lock("test-service")
	g.Unlock("test-service")

	result := g.Lock("test-service")
	assert.True(t, result)
}

func Test_Guard_Unlock_NotLocked(t *testing.T) {
	g := NewGuard()

	g.Unlock("test-service")

	result := g.Lock("test-service")
	assert.True(t, result)
}

func Test_Guard_IndependentServices(t *testing.T) {
	g := NewGuard()

	assert.True(t, g.Lock("service-a"))
	assert.True(t, g.Lock("service-b"))

	g.Unlock("service-a")

	assert.True(t, g.Lock("service-a"))
	assert.False(t, g.Lock("service-b"))
}

func Test_Guard_CanLockAfterUnlock(t *testing.T) {
	g := NewGuard()

	assert.True(t, g.Lock("test-service"))
	g.Unlock("test-service")

	assert.True(t, g.Lock("test-service"))
}
