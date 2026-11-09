package mcp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/instance"
	"fuku/internal/model"
)

// envProjectDir names the variable the agent host sets to the project directory
const envProjectDir = "CLAUDE_PROJECT_DIR"

// Names of the config file the default lookup tries in the project root, in its order, as fuku does
const (
	configDefault    = "fuku.yaml"
	configDefaultAlt = "fuku.yml"
)

// identify resolves the project of a call and the identity of its instance
func identify(config string) (model.Instance, error) {
	dir, err := projectDir(config)
	if err != nil {
		return model.Instance{}, err
	}

	return model.Instance{Project: dir, Fingerprint: instance.Fingerprint(dir)}, nil
}

// configFile returns the config file of a call inside its project directory, or empty for the default lookup
func configFile(config, dir string) string {
	if config == "" || rootConfig(config, dir) {
		return ""
	}

	return filepath.Join(dir, filepath.Base(config))
}

// rootConfig reports whether the config names the root file the default lookup picks, which loads with its override
func rootConfig(config, dir string) bool {
	rootDir, err := projectDir("")
	if err != nil || rootDir != dir {
		return false
	}

	base := filepath.Base(config)

	for _, name := range []string{configDefault, configDefaultAlt} {
		_, err := os.Stat(filepath.Join(rootDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		return err == nil && name == base
	}

	return false
}

// withConfig adds the config file of the call to the arguments of a fuku command, when the call names one
func withConfig(args []string, config, dir string) []string {
	file := configFile(config, dir)
	if file == "" {
		return args
	}

	return append(args, "--"+cli.FlagConfig.String(), file)
}

// projectDir resolves the project of a call: the directory of its config file under the root, or the root itself
func projectDir(config string) (string, error) {
	dir := root()

	switch {
	case filepath.IsAbs(config):
		dir = filepath.Dir(config)
	case config != "":
		dir = filepath.Join(dir, filepath.Dir(config))
	}

	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%w: %w", instance.ErrFailedToResolveProject, err)
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("%w: %w", instance.ErrFailedToResolveProject, err)
	}

	return resolved, nil
}

// root returns the project root: CLAUDE_PROJECT_DIR when set, the working directory otherwise
func root() string {
	if dir := os.Getenv(envProjectDir); dir != "" {
		return dir
	}

	return "."
}
