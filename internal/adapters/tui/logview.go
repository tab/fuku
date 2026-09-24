package tui

import (
	"io"

	"fuku/internal/adapters/terminal"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Formatter renders one service log line
type Formatter interface {
	FormatMessage(service, message string) string
}

// LogView writes the connection banner and the styled lines of a stream
type LogView struct {
	theme     terminal.Theme
	formatter Formatter
	out       io.Writer
	width     int
}

// NewLogView creates the inline log view for a terminal of the given width
func NewLogView(theme terminal.Theme, formatter Formatter, out io.Writer, width int) *LogView {
	return &LogView{
		theme:     theme,
		formatter: formatter,
		out:       out,
		width:     width,
	}
}

// Status writes the banner of the accepted stream
func (v *LogView) Status(status contracts.LogStatus, subscribed []string) {
	v.banner(status, subscribed)
}

// Line writes one formatted line
func (v *LogView) Line(line model.LogLine) {
	//nolint:errcheck // best-effort write to output
	io.WriteString(v.out, v.formatter.FormatMessage(line.Service, line.Message))
}
