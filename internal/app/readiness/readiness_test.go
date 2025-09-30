package readiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuku/internal/config"
)

func Test_Factory_CreateChecker_NilConfig(t *testing.T) {
	factoryInstance := NewFactory()
	checker, err := factoryInstance.CreateChecker(nil)
	assert.NoError(t, err)
	assert.Nil(t, checker)
}

func Test_Factory_CreateChecker_HTTPSuccess(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type:     "http",
		URL:      "http://localhost:8080/health",
		Timeout:  "10s",
		Interval: "1s",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.IsType(t, &HTTPChecker{}, checker)
}

func Test_Factory_CreateChecker_HTTPMissingURL(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type: "http",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	assert.Error(t, err)
	assert.Nil(t, checker)
	assert.Contains(t, err.Error(), "http readiness check requires url")
}

func Test_Factory_CreateChecker_LogSuccess(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type:    "log",
		Pattern: "server started",
		Timeout: "30s",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)
	assert.IsType(t, &LogChecker{}, checker)
}

func Test_Factory_CreateChecker_LogMissingPattern(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type: "log",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	assert.Error(t, err)
	assert.Nil(t, checker)
	assert.Contains(t, err.Error(), "log readiness check requires pattern")
}

func Test_Factory_CreateChecker_UnsupportedType(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type: "unsupported",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	assert.Error(t, err)
	assert.Nil(t, checker)
	assert.Contains(t, err.Error(), "unsupported readiness check type")
}

func Test_Factory_CreateChecker_DefaultValues(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type: "http",
		URL:  "http://localhost:8080/health",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)

	httpChecker := checker.(*HTTPChecker)
	assert.Equal(t, defaultTimeout, httpChecker.timeout)
	assert.Equal(t, defaultInterval, httpChecker.interval)
}

func Test_Factory_CreateChecker_CustomValues(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type:     "http",
		URL:      "http://localhost:8080/health",
		Timeout:  "5s",
		Interval: "250ms",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)
}

func Test_Factory_CreateChecker_InvalidTimeout(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type:    "http",
		URL:     "http://localhost:8080/health",
		Timeout: "invalid",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)

	httpChecker := checker.(*HTTPChecker)
	assert.Equal(t, defaultTimeout, httpChecker.timeout)
}

func Test_Factory_CreateChecker_InvalidInterval(t *testing.T) {
	factoryInstance := NewFactory()
	cfg := &config.ReadinessCheck{
		Type:     "http",
		URL:      "http://localhost:8080/health",
		Interval: "invalid",
	}

	checker, err := factoryInstance.CreateChecker(cfg)
	require.NoError(t, err)
	assert.NotNil(t, checker)

	httpChecker := checker.(*HTTPChecker)
	assert.Equal(t, defaultInterval, httpChecker.interval)
}
