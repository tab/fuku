package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MeterReExports_Functions(t *testing.T) {
	tests := []struct {
		name string
		fn   any
	}{
		{
			name: "NewMeter",
			fn:   NewMeter,
		},
		{
			name: "WithUnit",
			fn:   WithUnit,
		},
		{
			name: "WithAttributes",
			fn:   WithAttributes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.fn)
		})
	}
}

func Test_MeterReExports_AttributeBuilders(t *testing.T) {
	tests := []struct {
		name string
		fn   any
	}{
		{
			name: "StringAttr",
			fn:   StringAttr,
		},
		{
			name: "IntAttr",
			fn:   IntAttr,
		},
		{
			name: "BoolAttr",
			fn:   BoolAttr,
		},
		{
			name: "Float64Attr",
			fn:   Float64Attr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.fn)
		})
	}
}

func Test_MeterReExports_UnitConstants(t *testing.T) {
	assert.Equal(t, "millisecond", UnitMillisecond)
	assert.Equal(t, "second", UnitSecond)
}

func Test_MeterReExports_MetricNameConstants(t *testing.T) {
	tests := []struct {
		name   string
		metric string
	}{
		{
			name:   "MetricAppRun",
			metric: MetricAppRun,
		},
		{
			name:   "MetricServiceCount",
			metric: MetricServiceCount,
		},
		{
			name:   "MetricTierCount",
			metric: MetricTierCount,
		},
		{
			name:   "MetricDiscoveryDuration",
			metric: MetricDiscoveryDuration,
		},
		{
			name:   "MetricTierStartupDuration",
			metric: MetricTierStartupDuration,
		},
		{
			name:   "MetricServiceStartupDuration",
			metric: MetricServiceStartupDuration,
		},
		{
			name:   "MetricShutdownDuration",
			metric: MetricShutdownDuration,
		},
		{
			name:   "MetricReadinessDuration",
			metric: MetricReadinessDuration,
		},
		{
			name:   "MetricServiceFailed",
			metric: MetricServiceFailed,
		},
		{
			name:   "MetricUnexpectedExit",
			metric: MetricUnexpectedExit,
		},
		{
			name:   "MetricServiceRestart",
			metric: MetricServiceRestart,
		},
		{
			name:   "MetricWatchRestart",
			metric: MetricWatchRestart,
		},
		{
			name:   "MetricPreflightKilled",
			metric: MetricPreflightKilled,
		},
		{
			name:   "MetricPreflightDuration",
			metric: MetricPreflightDuration,
		},
		{
			name:   "MetricStartupDuration",
			metric: MetricStartupDuration,
		},
		{
			name:   "MetricFukuCPU",
			metric: MetricFukuCPU,
		},
		{
			name:   "MetricFukuMemory",
			metric: MetricFukuMemory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.metric)
		})
	}
}

func Test_MeterReExports_TagConstants(t *testing.T) {
	tests := []struct {
		name string
		tag  string
	}{
		{
			name: "TagArch",
			tag:  TagArch,
		},
		{
			name: "TagCommand",
			tag:  TagCommand,
		},
		{
			name: "TagEnv",
			tag:  TagEnv,
		},
		{
			name: "TagGoVersion",
			tag:  TagGoVersion,
		},
		{
			name: "TagOS",
			tag:  TagOS,
		},
		{
			name: "TagProfile",
			tag:  TagProfile,
		},
		{
			name: "TagServiceCount",
			tag:  TagServiceCount,
		},
		{
			name: "TagType",
			tag:  TagType,
		},
		{
			name: "TagUI",
			tag:  TagUI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.tag)
		})
	}
}
