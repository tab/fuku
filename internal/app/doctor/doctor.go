// Package doctor diagnoses configuration, environment, and runtime issues for fuku
package doctor

import "time"

// Status is the result status of a single check
type Status int

// Status values, ordered by severity from healthy to fatal
const (
	StatusOK   Status = iota // ok
	StatusIdle               // inactive but expected
	StatusNote               // informational
	StatusWarn               // non-fatal issue
	StatusFail               // fatal issue
)

// String returns the lowercase name of a Status (used in JSON output)
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusIdle:
		return "idle"
	case StatusNote:
		return "note"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

// Glyph returns the single-character indicator for a Status
func (s Status) Glyph() string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusIdle:
		return "○"
	case StatusNote:
		return "↑"
	case StatusWarn:
		return "⚠"
	case StatusFail:
		return "✗"
	default:
		return "?"
	}
}

// Detail is a key-value pair shown in the indented detail block of a result
type Detail struct {
	Key   string
	Value string
}

// Result is the outcome of a single check
type Result struct {
	ID          string
	Category    string
	Status      Status
	Summary     string
	Details     []Detail
	Remediation string
	DurationMs  int64
}

// Section groups results under a category heading
type Section struct {
	Title   string
	Note    string
	Results []Result
}

// Report is the full output of a doctor run
type Report struct {
	SchemaVersion int
	GeneratedAt   time.Time
	FukuVersion   string
	Platform      string
	Sections      []Section
}

// Tally aggregates result counts by status
type Tally struct {
	OK   int
	Idle int
	Note int
	Warn int
	Fail int
}

// Tally counts results by status across all sections
func (r *Report) Tally() Tally {
	var t Tally

	for _, s := range r.Sections {
		for _, res := range s.Results {
			switch res.Status {
			case StatusOK:
				t.OK++
			case StatusIdle:
				t.Idle++
			case StatusNote:
				t.Note++
			case StatusWarn:
				t.Warn++
			case StatusFail:
				t.Fail++
			}
		}
	}

	return t
}

// OverallStatus returns the worst status across the whole report
func (r *Report) OverallStatus() Status {
	t := r.Tally()

	switch {
	case t.Fail > 0:
		return StatusFail
	case t.Warn > 0:
		return StatusWarn
	case t.Note > 0:
		return StatusNote
	default:
		return StatusOK
	}
}

// ExitCode returns the process exit code for the report (0=ok or warn, 2=fail)
func (r *Report) ExitCode() int {
	if r.Tally().Fail > 0 {
		return 2
	}

	return 0
}

// Notes returns the non-OK results across all sections, in display order
func (r *Report) Notes() []Result {
	var notes []Result

	for _, s := range r.Sections {
		for _, res := range s.Results {
			if res.Status != StatusOK && res.Status != StatusIdle {
				notes = append(notes, res)
			}
		}
	}

	return notes
}
