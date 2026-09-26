package doctor

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
)

func Test_newReport(t *testing.T) {
	report := newReport("0.99.0")

	assert.Equal(t, 1, report.SchemaVersion)
	assert.Equal(t, "0.99.0", report.Version)
	assert.Equal(t, runtime.GOOS+"-"+runtime.GOARCH, report.Platform)
	assert.False(t, report.GeneratedAt.IsZero())
	assert.Nil(t, report.Sections)
}

func Test_Overall(t *testing.T) {
	tests := []struct {
		name     string
		results  []model.Result
		expected model.Severity
	}{
		{
			name:     "all ok",
			results:  []model.Result{{Severity: model.SeverityOK}, {Severity: model.SeverityOK}},
			expected: model.SeverityOK,
		},
		{
			name:     "with idle",
			results:  []model.Result{{Severity: model.SeverityOK}, {Severity: model.SeverityIdle}},
			expected: model.SeverityOK,
		},
		{
			name:     "with note",
			results:  []model.Result{{Severity: model.SeverityOK}, {Severity: model.SeverityNote}},
			expected: model.SeverityNote,
		},
		{
			name:     "with warn",
			results:  []model.Result{{Severity: model.SeverityOK}, {Severity: model.SeverityWarn}, {Severity: model.SeverityNote}},
			expected: model.SeverityWarn,
		},
		{
			name:     "with fail",
			results:  []model.Result{{Severity: model.SeverityOK}, {Severity: model.SeverityWarn}, {Severity: model.SeverityFail}},
			expected: model.SeverityFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &model.Report{Sections: []model.Section{{Results: tt.results}}}

			assert.Equal(t, tt.expected, Overall(report))
		})
	}
}

func Test_Notes(t *testing.T) {
	report := &model.Report{
		Sections: []model.Section{
			{Results: []model.Result{
				{ID: "a", Severity: model.SeverityOK},
				{ID: "b", Severity: model.SeverityIdle},
				{ID: "c", Severity: model.SeverityWarn},
			}},
			{Results: []model.Result{
				{ID: "d", Severity: model.SeverityFail},
				{ID: "e", Severity: model.SeverityNote},
			}},
		},
	}

	notes := Notes(report)

	assert.Equal(t, []model.Result{
		{ID: "c", Severity: model.SeverityWarn},
		{ID: "d", Severity: model.SeverityFail},
		{ID: "e", Severity: model.SeverityNote},
	}, notes)
}
