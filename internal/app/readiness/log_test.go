package readiness

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/app/errors"
)

func Test_LogChecker_Check_Success(t *testing.T) {
	checker, err := NewLogChecker("server started", 5*time.Second)
	require.NoError(t, err)

	ctx := context.Background()

	go func() {
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("initializing...")
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("server started on port 8080")
	}()

	err = checker.Check(ctx)
	assert.NoError(t, err)
}

func Test_LogChecker_Check_Timeout(t *testing.T) {
	checker, err := NewLogChecker("ready", 300*time.Millisecond)
	require.NoError(t, err)

	ctx := context.Background()

	go func() {
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("starting...")
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("initializing...")
	}()

	err = checker.Check(ctx)
	assert.Equal(t, errors.ErrReadinessCheckFailed, err)
}

func Test_LogChecker_Check_ContextCancelled(t *testing.T) {
	checker, err := NewLogChecker("ready", 5*time.Second)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err = checker.Check(ctx)
	assert.Equal(t, context.Canceled, err)
}

func Test_LogChecker_Check_RegexPattern(t *testing.T) {
	checker, err := NewLogChecker("listening on port [0-9]+", 5*time.Second)
	require.NoError(t, err)

	ctx := context.Background()

	go func() {
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("starting server...")
		time.Sleep(100 * time.Millisecond)
		checker.AddLogLine("listening on port 8080")
	}()

	err = checker.Check(ctx)
	assert.NoError(t, err)
}

func Test_LogChecker_Check_NoMatch(t *testing.T) {
	checker, err := NewLogChecker("ready", 300*time.Millisecond)
	require.NoError(t, err)

	ctx := context.Background()

	go func() {
		checker.AddLogLine("starting...")
		checker.AddLogLine("initializing...")
		checker.AddLogLine("not the pattern you're looking for")
	}()

	err = checker.Check(ctx)
	assert.Equal(t, errors.ErrReadinessCheckFailed, err)
}

func Test_NewLogChecker_InvalidPattern(t *testing.T) {
	_, err := NewLogChecker("[invalid(", 5*time.Second)
	assert.Equal(t, errors.ErrInvalidRegexPattern, err)
}

func Test_LogChecker_AddLogLine_AfterDone(t *testing.T) {
	checker, err := NewLogChecker("ready", 100*time.Millisecond)
	require.NoError(t, err)

	ctx := context.Background()

	go func() {
		time.Sleep(50 * time.Millisecond)
		checker.AddLogLine("ready")
	}()

	err = checker.Check(ctx)
	assert.NoError(t, err)

	checker.AddLogLine("this should not panic")
}
