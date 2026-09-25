package tui

import (
	"fmt"
	"io"
	"strings"

	"fuku/internal/adapters/terminal"
	"fuku/internal/app/doctor"
	"fuku/internal/model"
)

const (
	divider        = "┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄"
	idColumnWidth  = 12
	detailKeyWidth = 25
)

// Report renders the styled default report inline: the notes, every section with its details and the tally
type Report struct {
	theme func() terminal.Theme
}

// NewReport creates the styled report renderer for the terminal's theme
func NewReport(theme func() terminal.Theme) *Report {
	return &Report{theme: theme}
}

// Render writes the verbose grouped report to w
func (r *Report) Render(w io.Writer, report *model.Report) error {
	theme := r.theme()

	writeHeader(w, report)
	writeNotes(w, theme, report)

	for _, section := range report.Sections {
		writeSection(w, theme, section, true)
	}

	writeFooter(w, report)

	return nil
}

// Summary renders the styled compact report inline: one line per check and the tally
type Summary struct {
	theme func() terminal.Theme
}

// NewSummary creates the styled summary renderer for the terminal's theme
func NewSummary(theme func() terminal.Theme) *Summary {
	return &Summary{theme: theme}
}

// Render writes the header, one row per check and the tally to w
func (s *Summary) Render(w io.Writer, report *model.Report) error {
	theme := s.theme()

	writeHeader(w, report)

	for _, section := range report.Sections {
		writeSection(w, theme, section, false)
	}

	writeFooter(w, report)

	return nil
}

// writeHeader prints the title line and a blank line
func writeHeader(w io.Writer, r *model.Report) {
	fmt.Fprintf(w, "fuku doctor v%s · %s\n\n", r.Version, r.Platform)
}

// writeNotes prints the highlighted non-OK results at the top of the report
func writeNotes(w io.Writer, theme terminal.Theme, r *model.Report) {
	notes := doctor.Notes(r)
	if len(notes) == 0 {
		return
	}

	fmt.Fprintln(w, "Notes")

	for _, res := range notes {
		fmt.Fprintf(w, "   %s %s %s\n", styledGlyph(theme, res.Severity), padRight(string(res.ID), idColumnWidth), res.Summary)
	}

	fmt.Fprintln(w, divider)
}

// writeSection prints a section header and its results
func writeSection(w io.Writer, theme terminal.Theme, s model.Section, withDetails bool) {
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
		writeResult(w, theme, res, withDetails)
	}
}

// writeResult prints a single result row and its optional indented details
func writeResult(w io.Writer, theme terminal.Theme, res model.Result, withDetails bool) {
	fmt.Fprintf(w, "  %s %s %s\n", styledGlyph(theme, res.Severity), padRight(string(res.ID), idColumnWidth), res.Summary)

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

// writeFooter prints the divider and the tally line
func writeFooter(w io.Writer, r *model.Report) {
	t := r.Tally()

	fmt.Fprintln(w)
	fmt.Fprintln(w, divider)
	fmt.Fprintf(w, "%d ok · %d idle · %d notes · %d warn · %d fail\n",
		t.OK, t.Idle, t.Note, t.Warn, t.Fail)
}

// styledGlyph returns the severity glyph styled with its severity color from the theme
func styledGlyph(theme terminal.Theme, s model.Severity) string {
	switch s {
	case model.SeverityOK:
		return theme.DoctorGlyphOKStyle.Render("✓")
	case model.SeverityIdle:
		return theme.DoctorGlyphIdleStyle.Render("○")
	case model.SeverityNote:
		return theme.DoctorGlyphNoteStyle.Render("↑")
	case model.SeverityWarn:
		return theme.DoctorGlyphWarnStyle.Render("⚠")
	case model.SeverityFail:
		return theme.DoctorGlyphFailStyle.Render("✗")
	default:
		return "?"
	}
}

// padRight returns s padded with spaces on the right to at least width runes
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}

	return s + strings.Repeat(" ", width-len(s))
}
