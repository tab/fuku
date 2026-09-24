package diagnostics

import (
	"os"
	"os/exec"

	"fuku/internal/platform/buildinfo"
)

// Environment observes the process environment and the fuku installation
type Environment struct{}

// NewEnvironment creates an observer of the process environment
func NewEnvironment() *Environment {
	return &Environment{}
}

// Getenv returns the value of the named environment variable, or empty when it is unset
func (e *Environment) Getenv(key string) string {
	return os.Getenv(key)
}

// Executable returns the path of the running fuku binary
func (e *Environment) Executable() (string, error) {
	return os.Executable()
}

// PathExecutable returns the fuku binary a PATH lookup resolves to
func (e *Environment) PathExecutable() (string, error) {
	return exec.LookPath(buildinfo.AppName)
}
