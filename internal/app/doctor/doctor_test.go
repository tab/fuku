package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Status_String(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "ok",
			status:   StatusOK,
			expected: "ok",
		},
		{
			name:     "idle",
			status:   StatusIdle,
			expected: "idle",
		},
		{
			name:     "note",
			status:   StatusNote,
			expected: "note",
		},
		{
			name:     "warn",
			status:   StatusWarn,
			expected: "warn",
		},
		{
			name:     "fail",
			status:   StatusFail,
			expected: "fail",
		},
		{
			name:     "unknown",
			status:   Status(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func Test_Status_Glyph(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "ok",
			status:   StatusOK,
			expected: "✓",
		},
		{
			name:     "idle",
			status:   StatusIdle,
			expected: "○",
		},
		{
			name:     "note",
			status:   StatusNote,
			expected: "↑",
		},
		{
			name:     "warn",
			status:   StatusWarn,
			expected: "⚠",
		},
		{
			name:     "fail",
			status:   StatusFail,
			expected: "✗",
		},
		{
			name:     "unknown",
			status:   Status(99),
			expected: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.Glyph())
		})
	}
}

func Test_Report_Tally(t *testing.T) {
	report := &Report{
		Sections: []Section{
			{Results: []Result{
				{Status: StatusOK},
				{Status: StatusOK},
				{Status: StatusWarn},
			}},
			{Results: []Result{
				{Status: StatusFail},
				{Status: StatusIdle},
				{Status: StatusNote},
			}},
		},
	}

	tally := report.Tally()

	assert.Equal(t, 2, tally.OK)
	assert.Equal(t, 1, tally.Idle)
	assert.Equal(t, 1, tally.Note)
	assert.Equal(t, 1, tally.Warn)
	assert.Equal(t, 1, tally.Fail)
}

func Test_Report_OverallStatus(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		expected Status
	}{
		{
			name:     "all ok",
			results:  []Result{{Status: StatusOK}, {Status: StatusOK}},
			expected: StatusOK,
		},
		{
			name:     "with idle",
			results:  []Result{{Status: StatusOK}, {Status: StatusIdle}},
			expected: StatusOK,
		},
		{
			name:     "with note",
			results:  []Result{{Status: StatusOK}, {Status: StatusNote}},
			expected: StatusNote,
		},
		{
			name:     "with warn",
			results:  []Result{{Status: StatusOK}, {Status: StatusWarn}, {Status: StatusNote}},
			expected: StatusWarn,
		},
		{
			name:     "with fail",
			results:  []Result{{Status: StatusOK}, {Status: StatusWarn}, {Status: StatusFail}},
			expected: StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Sections: []Section{{Results: tt.results}}}
			assert.Equal(t, tt.expected, report.OverallStatus())
		})
	}
}

func Test_Report_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		expected int
	}{
		{
			name:     "all ok",
			results:  []Result{{Status: StatusOK}},
			expected: 0,
		},
		{
			name:     "with warn but no fail",
			results:  []Result{{Status: StatusWarn}},
			expected: 0,
		},
		{
			name:     "with fail",
			results:  []Result{{Status: StatusFail}},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Sections: []Section{{Results: tt.results}}}
			assert.Equal(t, tt.expected, report.ExitCode())
		})
	}
}

func Test_Report_Notes(t *testing.T) {
	report := &Report{
		Sections: []Section{
			{Results: []Result{
				{ID: "a", Status: StatusOK},
				{ID: "b", Status: StatusIdle},
				{ID: "c", Status: StatusWarn},
			}},
			{Results: []Result{
				{ID: "d", Status: StatusFail},
				{ID: "e", Status: StatusNote},
			}},
		},
	}

	notes := report.Notes()

	assert.Len(t, notes, 3)
	assert.Equal(t, "c", notes[0].ID)
	assert.Equal(t, "d", notes[1].ID)
	assert.Equal(t, "e", notes[2].ID)
}

func Test_Report_GeneratedAt(t *testing.T) {
	report := &Report{GeneratedAt: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)}

	assert.Equal(t, "2026-06-06T12:00:00Z", report.GeneratedAt.Format("2006-01-02T15:04:05Z"))
}
