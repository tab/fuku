package sentry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TraceReExports_Functions(t *testing.T) {
	tests := []struct {
		name string
		fn   any
	}{
		{
			name: "StartTransaction",
			fn:   StartTransaction,
		},
		{
			name: "WithTransactionSource",
			fn:   WithTransactionSource,
		},
		{
			name: "WithDescription",
			fn:   WithDescription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.fn)
		})
	}
}

func Test_TraceReExports_SourceTask(t *testing.T) {
	assert.Equal(t, "task", string(SourceTask))
}

func Test_TraceReExports_SpanStatusConstants(t *testing.T) {
	assert.NotEqual(t, SpanStatusOK, SpanStatusCanceled)
}

func Test_TraceReExports_SpanOperationConstants(t *testing.T) {
	tests := []struct {
		name string
		op   string
	}{
		{
			name: "OpDiscovery",
			op:   OpDiscovery,
		},
		{
			name: "OpPreflight",
			op:   OpPreflight,
		},
		{
			name: "OpTierStartup",
			op:   OpTierStartup,
		},
		{
			name: "OpShutdown",
			op:   OpShutdown,
		},
		{
			name: "OpWatchRestart",
			op:   OpWatchRestart,
		},
		{
			name: "OpServiceStop",
			op:   OpServiceStop,
		},
		{
			name: "OpServiceRestart",
			op:   OpServiceRestart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.op)
		})
	}
}
