package cli

import (
	"context"
	"io"

	"fuku/internal/app/doctor"
	"fuku/internal/model"
)

// Exit codes of the doctor command
const (
	exitOK    = 0
	exitFail  = 2
	exitError = 3
)

// Checker runs the doctor checks
type Checker interface {
	Run(ctx context.Context) *model.Report
}

// Renderer writes a report in one output format
type Renderer interface {
	Render(w io.Writer, r *model.Report) error
}

// Doctor runs the doctor checks and writes the report through the selected renderer
type Doctor struct {
	checker  Checker
	renderer Renderer
	stdout   io.Writer
}

// NewDoctor creates the doctor command writing the report to stdout
func NewDoctor(checker Checker, renderer Renderer, stdout io.Writer) *Doctor {
	return &Doctor{
		checker:  checker,
		renderer: renderer,
		stdout:   stdout,
	}
}

// Run writes the report and returns the exit code (0 without failures, 2 with one, 3 when writing fails)
func (c *Doctor) Run(ctx context.Context) (int, error) {
	report := c.checker.Run(ctx)

	if err := c.renderer.Render(c.stdout, report); err != nil {
		return exitError, err
	}

	return exitCode(report), nil
}

// exitCode maps the report to the process exit code (a warn still exits 0, a fail exits 2)
func exitCode(r *model.Report) int {
	if doctor.Overall(r) == model.SeverityFail {
		return exitFail
	}

	return exitOK
}
