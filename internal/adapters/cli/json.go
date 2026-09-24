package cli

import (
	"encoding/json"
	"io"

	"fuku/internal/app/doctor"
	"fuku/internal/model"
)

// JSON renders the machine-readable report
type JSON struct{}

// NewJSON creates the JSON renderer
func NewJSON() *JSON {
	return &JSON{}
}

// Render writes the indented JSON report to w
func (j *JSON) Render(w io.Writer, r *model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(toJSONReport(r))
}

// jsonReport is the stable JSON schema for --json output
type jsonReport struct {
	SchemaVersion int                    `json:"schemaVersion"`
	GeneratedAt   string                 `json:"generatedAt"`
	FukuVersion   string                 `json:"fukuVersion"`
	Platform      string                 `json:"platform"`
	OverallStatus string                 `json:"overallStatus"`
	Tally         jsonTally              `json:"tally"`
	Checks        map[string]jsonCheck   `json:"checks"`
	Sections      []jsonSectionReference `json:"sections"`
}

type jsonTally struct {
	OK   int `json:"ok"`
	Idle int `json:"idle"`
	Note int `json:"note"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

type jsonCheck struct {
	ID          string            `json:"id"`
	Category    string            `json:"category"`
	Status      string            `json:"status"`
	Summary     string            `json:"summary"`
	Details     map[string]string `json:"details,omitempty"`
	Remediation string            `json:"remediation,omitempty"`
	DurationMs  int64             `json:"durationMs"`
}

type jsonSectionReference struct {
	Title  string   `json:"title"`
	Note   string   `json:"note,omitempty"`
	Checks []string `json:"checks"`
}

// toJSONReport converts a report to the wire JSON representation
func toJSONReport(r *model.Report) jsonReport {
	out := jsonReport{
		SchemaVersion: r.SchemaVersion,
		GeneratedAt:   r.GeneratedAt.Format("2006-01-02T15:04:05Z"),
		FukuVersion:   r.Version,
		Platform:      r.Platform,
		OverallStatus: doctor.Overall(r).String(),
		Tally:         jsonTally(r.Tally()),
		Checks:        map[string]jsonCheck{},
	}

	for _, section := range r.Sections {
		ref := jsonSectionReference{Title: section.Title, Note: section.Note}

		for _, res := range section.Results {
			out.Checks[string(res.ID)] = jsonCheck{
				ID:          string(res.ID),
				Category:    string(res.Category),
				Status:      res.Severity.String(),
				Summary:     res.Summary,
				Details:     detailsToMap(res.Details),
				Remediation: res.Remediation,
				DurationMs:  res.Duration.Milliseconds(),
			}
			ref.Checks = append(ref.Checks, string(res.ID))
		}

		out.Sections = append(out.Sections, ref)
	}

	return out
}

// detailsToMap converts details to a string map, returning nil for empty input
func detailsToMap(details []model.Detail) map[string]string {
	if len(details) == 0 {
		return nil
	}

	m := make(map[string]string, len(details))
	for _, d := range details {
		m[d.Key] = d.Value
	}

	return m
}
