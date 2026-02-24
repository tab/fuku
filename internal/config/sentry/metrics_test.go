package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MeterReExports(t *testing.T) {
	t.Run("NewMeter is not nil", func(t *testing.T) {
		assert.NotNil(t, NewMeter)
	})

	t.Run("WithUnit is not nil", func(t *testing.T) {
		assert.NotNil(t, WithUnit)
	})

	t.Run("WithAttributes is not nil", func(t *testing.T) {
		assert.NotNil(t, WithAttributes)
	})

	t.Run("Attribute builders are not nil", func(t *testing.T) {
		assert.NotNil(t, StringAttr)
		assert.NotNil(t, IntAttr)
		assert.NotNil(t, BoolAttr)
		assert.NotNil(t, Float64Attr)
	})

	t.Run("Unit constants match expected values", func(t *testing.T) {
		assert.Equal(t, "millisecond", UnitMillisecond)
		assert.Equal(t, "second", UnitSecond)
	})

	t.Run("Metric name constants are not empty", func(t *testing.T) {
		metrics := []string{
			MetricAppRun,
			MetricServiceCount,
			MetricTierCount,
			MetricDiscoveryDuration,
			MetricTierStartupDuration,
			MetricServiceStartupDuration,
			MetricShutdownDuration,
			MetricReadinessDuration,
			MetricServiceFailed,
			MetricUnexpectedExit,
			MetricServiceRestart,
			MetricWatchRestart,
			MetricPreflightKilled,
		}

		for _, name := range metrics {
			assert.NotEmpty(t, name)
		}
	})
}
