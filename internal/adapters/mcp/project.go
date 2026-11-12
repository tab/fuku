package mcp

import (
	"os"
	"path/filepath"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/instance"
	"fuku/internal/model"
)

// envProjectDir names the variable the agent host sets to the project directory
const envProjectDir = "CLAUDE_PROJECT_DIR"

// identify resolves the project of a call and the identity of its instance
func identify(config string) (model.Instance, error) {
	dir, err := projectDir(config)
	if err != nil {
		return model.Instance{}, err
	}

	return model.Instance{Project: dir, Fingerprint: instance.Fingerprint(dir)}, nil
}

// configName returns the file the config of a call names inside its project directory, or empty for the default lookup
func configName(config string) string {
	if config == "" {
		return ""
	}

	return filepath.Base(config)
}

// withConfig adds the config file of the call to the arguments of a fuku command, unless the default lookup reads it
func (s *Server) withConfig(args []string, config, dir string) []string {
	file := filepath.Join(dir, configName(config))
	if config == "" || file == s.load(dir, "").Path {
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

	return instance.Project(dir)
}

// root returns the project root: CLAUDE_PROJECT_DIR when set, the working directory otherwise
func root() string {
	if dir := os.Getenv(envProjectDir); dir != "" {
		return dir
	}

	return "."
}
