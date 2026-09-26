package doctor

import (
	"time"

	"fuku/internal/model"
)

// timed records the duration of fn into the returned result
func timed(fn func() model.Result) model.Result {
	start := time.Now()
	r := fn()
	r.Duration = time.Since(start)

	return r
}

// skipped builds the idle placeholder of a check that could not run for the given reason
func skipped(id model.CheckID, category model.Category, reason string) model.Result {
	return model.Result{
		ID:       id,
		Category: category,
		Severity: model.SeverityIdle,
		Summary:  "skipped (" + reason + ")",
	}
}
