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
	theme     func() terminal.Theme
	formatter Formatter
	out       io.Writer
}

// NewLogView creates the inline log view
func NewLogView(theme func() terminal.Theme, formatter Formatter, out io.Writer) *LogView {
	return &LogView{
		theme:     theme,
		formatter: formatter,
		out:       out,
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
