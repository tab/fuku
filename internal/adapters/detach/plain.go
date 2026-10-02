package detach

import (
	"fmt"
	"io"
	"time"
)

// Plain renders the startup as one line per event, free of escape sequences, for a pipe or --no-ui
type Plain struct {
	stdout io.Writer
}

// NewPlain creates the plain view
func NewPlain(stdout io.Writer) *Plain {
	return &Plain{stdout: stdout}
}

// Open has nothing to prepare
func (p *Plain) Open() {}

// Show writes the line of one record
func (p *Plain) Show(record Record) {
	switch record.Kind {
	case KindStarting:
		fmt.Fprintf(p.stdout, " • %s Starting\n", record.Service)
	case KindReady:
		fmt.Fprintf(p.stdout, " ✔ %s Ready %s\n", record.Service, seconds(record.Duration))
	case KindFailed:
		fmt.Fprintf(p.stdout, " ✗ %s Failed: %s\n", record.Service, record.Error)
	default:
		// no-op: the summary reports the API and the running instance
	}
}

// Close has nothing to release
func (p *Plain) Close() {}

// seconds formats a duration the way docker compose does, in seconds with one decimal
func seconds(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
