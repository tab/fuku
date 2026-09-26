package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Severity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected string
	}{
		{
			name:     "ok",
			severity: SeverityOK,
			expected: "ok",
		},
		{
			name:     "idle",
			severity: SeverityIdle,
			expected: "idle",
		},
		{
			name:     "note",
			severity: SeverityNote,
			expected: "note",
		},
		{
			name:     "warn",
			severity: SeverityWarn,
			expected: "warn",
		},
		{
			name:     "fail",
			severity: SeverityFail,
			expected: "fail",
		},
		{
			name:     "unknown",
			severity: Severity(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.severity.String()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Report_Tally(t *testing.T) {
	report := &Report{
		Sections: []Section{
			{Results: []Result{
				{Severity: SeverityOK},
				{Severity: SeverityOK},
				{Severity: SeverityWarn},
			}},
			{Results: []Result{
				{Severity: SeverityFail},
				{Severity: SeverityIdle},
				{Severity: SeverityNote},
			}},
		},
	}

	tally := report.Tally()

	assert.Equal(t, Tally{OK: 2, Idle: 1, Note: 1, Warn: 1, Fail: 1}, tally)
}
