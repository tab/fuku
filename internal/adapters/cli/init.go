package cli

import (
	"context"
	"io"
)

// Init writes the fuku.yaml template into the working directory through the config adapter's creator
type Init struct {
	create func(stdout io.Writer) (int, error)
	stdout io.Writer
}

// NewInit creates the init command around the template creator
func NewInit(create func(stdout io.Writer) (int, error), stdout io.Writer) *Init {
	return &Init{create: create, stdout: stdout}
}

// Run creates the template and returns the exit code (1 when it cannot be written, with the reason as the error)
func (i *Init) Run(context.Context) (int, error) {
	return i.create(i.stdout)
}
