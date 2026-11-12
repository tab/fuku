package mcp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Binary runs the fuku binary this server runs from
type Binary struct{}

// NewBinary creates the runner of the fuku binary
func NewBinary() *Binary {
	return &Binary{}
}

// Execute runs the binary in dir with no stdin and returns its stdout, also when it exits non-zero
func (b *Binary) Execute(ctx context.Context, dir string, args ...string) ([]byte, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to locate the fuku binary: %w", err)
	}

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("fuku %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}
