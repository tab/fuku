package cli

import (
	"io"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Formatter renders one service log line
type Formatter interface {
	FormatMessage(service, message string) string
}

// LogView writes the stream as bare lines, without a banner
type LogView struct {
	formatter Formatter
	out       io.Writer
}

// NewLogView creates the bare log view
func NewLogView(formatter Formatter, out io.Writer) *LogView {
	return &LogView{formatter: formatter, out: out}
}

// Status ignores the accepted stream, which the bare view does not announce
func (v *LogView) Status(contracts.LogStatus, []string) {}

// Line writes one formatted line
func (v *LogView) Line(line model.LogLine) {
	//nolint:errcheck // best-effort write to output
	io.WriteString(v.out, v.formatter.FormatMessage(line.Service, line.Message))
}
