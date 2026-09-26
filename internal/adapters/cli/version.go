package cli

import (
	"context"
	"fmt"
	"io"

	"fuku/internal/platform/buildinfo"
)

// Version prints the build version
type Version struct {
	stdout io.Writer
}

// NewVersion creates the version command
func NewVersion(stdout io.Writer) *Version {
	return &Version{stdout: stdout}
}

// Run prints the version and returns the exit code
func (v *Version) Run(context.Context) (int, error) {
	fmt.Fprintf(v.stdout, "Version: %s\n", buildinfo.Version)

	return 0, nil
}
