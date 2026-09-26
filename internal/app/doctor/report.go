package doctor

import (
	"runtime"
	"time"

	"fuku/internal/model"
)

// schemaVersion is the version of the JSON report shape
const schemaVersion = 1

// newReport starts a report stamped with the schema version, the time, the fuku version and the platform
func newReport(version string) *model.Report {
	return &model.Report{
		SchemaVersion: schemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Version:       version,
		Platform:      runtime.GOOS + "-" + runtime.GOARCH,
	}
}

// Overall returns the worst severity across the whole report
func Overall(r *model.Report) model.Severity {
	t := r.Tally()

	switch {
	case t.Fail > 0:
		return model.SeverityFail
	case t.Warn > 0:
		return model.SeverityWarn
	case t.Note > 0:
		return model.SeverityNote
	default:
		return model.SeverityOK
	}
}

// Notes returns the results that need attention (neither ok nor idle) across all sections, in display order
func Notes(r *model.Report) []model.Result {
	var notes []model.Result

	for _, s := range r.Sections {
		for _, res := range s.Results {
			if res.Severity != model.SeverityOK && res.Severity != model.SeverityIdle {
				notes = append(notes, res)
			}
		}
	}

	return notes
}
