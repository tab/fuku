package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"fuku/internal/app/ui/components"
)

const (
	divider        = "┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄"
	idColumnWidth  = 12
	detailKeyWidth = 25
)

// RenderText writes the verbose grouped report to w
func RenderText(w io.Writer, r *Report) {
	writeHeader(w, r)
	writeNotes(w, r)

	for _, section := range r.Sections {
		writeSection(w, section, true)
	}

	writeFooter(w, r)
}

// RenderSummary writes a compact one-line-per-check report to w
func RenderSummary(w io.Writer, r *Report) {
	writeHeader(w, r)

	for _, section := range r.Sections {
		writeSection(w, section, false)
	}

	writeFooter(w, r)
}

// RenderJSON writes a machine-readable JSON report to w
func RenderJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(toJSONReport(r))
}

// writeHeader prints the title line and a blank line
func writeHeader(w io.Writer, r *Report) {
	fmt.Fprintf(w, "fuku doctor v%s · %s\n\n", r.FukuVersion, r.Platform)
}

// writeNotes prints the highlighted non-OK results at the top of the report
func writeNotes(w io.Writer, r *Report) {
	notes := r.Notes()
	if len(notes) == 0 {
		return
	}

	fmt.Fprintln(w, "Notes")

	for _, res := range notes {
		fmt.Fprintf(w, "   %s %s %s\n", styledGlyph(res.Status), padRight(res.ID, idColumnWidth), res.Summary)
	}

	fmt.Fprintln(w, divider)
}

// writeSection prints a section header and its results
func writeSection(w io.Writer, s Section, withDetails bool) {
	if len(s.Results) == 0 {
		return
	}

	fmt.Fprintln(w)

	if s.Note != "" {
		fmt.Fprintf(w, "%s · %s\n", s.Title, s.Note)
	} else {
		fmt.Fprintln(w, s.Title)
	}

	for _, res := range s.Results {
		writeResult(w, res, withDetails)
	}
}

// writeResult prints a single result row and its optional indented details
func writeResult(w io.Writer, res Result, withDetails bool) {
	fmt.Fprintf(w, "  %s %s %s\n", styledGlyph(res.Status), padRight(res.ID, idColumnWidth), res.Summary)

	if !withDetails {
		return
	}

	for _, detail := range res.Details {
		fmt.Fprintf(w, "      %s %s\n", padRight(detail.Key, detailKeyWidth), detail.Value)
	}

	if res.Remediation != "" {
		fmt.Fprintf(w, "      %s %s\n", padRight("remediation", detailKeyWidth), res.Remediation)
	}
}

// styledGlyph returns the status glyph styled with its status color
func styledGlyph(s Status) string {
	glyph := s.Glyph()

	switch s {
	case StatusOK:
		return components.DoctorGlyphOKStyle.Render(glyph)
	case StatusIdle:
		return components.DoctorGlyphIdleStyle.Render(glyph)
	case StatusNote:
		return components.DoctorGlyphNoteStyle.Render(glyph)
	case StatusWarn:
		return components.DoctorGlyphWarnStyle.Render(glyph)
	case StatusFail:
		return components.DoctorGlyphFailStyle.Render(glyph)
	default:
		return glyph
	}
}

// writeFooter prints the divider and the tally line
func writeFooter(w io.Writer, r *Report) {
	t := r.Tally()

	fmt.Fprintln(w)
	fmt.Fprintln(w, divider)
	fmt.Fprintf(w, "%d ok · %d idle · %d notes · %d warn · %d fail\n",
		t.OK, t.Idle, t.Note, t.Warn, t.Fail)
}

// padRight returns s padded with spaces on the right to at least width runes
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}

	return s + strings.Repeat(" ", width-len(s))
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

// toJSONReport converts a Report to the wire JSON representation
func toJSONReport(r *Report) jsonReport {
	out := jsonReport{
		SchemaVersion: r.SchemaVersion,
		GeneratedAt:   r.GeneratedAt.Format("2006-01-02T15:04:05Z"),
		FukuVersion:   r.FukuVersion,
		Platform:      r.Platform,
		OverallStatus: r.OverallStatus().String(),
		Tally:         jsonTally(r.Tally()),
		Checks:        map[string]jsonCheck{},
	}

	for _, section := range r.Sections {
		ref := jsonSectionReference{Title: section.Title, Note: section.Note}

		for _, res := range section.Results {
			out.Checks[res.ID] = jsonCheck{
				ID:          res.ID,
				Category:    res.Category,
				Status:      res.Status.String(),
				Summary:     res.Summary,
				Details:     detailsToMap(res.Details),
				Remediation: res.Remediation,
				DurationMs:  res.DurationMs,
			}
			ref.Checks = append(ref.Checks, res.ID)
		}

		out.Sections = append(out.Sections, ref)
	}

	return out
}

// detailsToMap converts details to a string map, returning nil for empty input
func detailsToMap(details []Detail) map[string]string {
	if len(details) == 0 {
		return nil
	}

	m := make(map[string]string, len(details))
	for _, d := range details {
		m[d.Key] = d.Value
	}

	return m
}
