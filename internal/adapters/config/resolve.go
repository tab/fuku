package config

import (
	"path/filepath"

	"fuku/internal/model"
)

// LoadPath reads path alone, or the default file of the working directory with its override when path is empty
func LoadPath(path string) model.Config {
	if path == "" {
		return modelConfig(loadDefault())
	}

	return modelConfig(loadFromFile(path))
}

// LoadDir reads the default file of dir with its override when path is empty or names it, and path alone otherwise
func LoadDir(dir, path string) model.Config {
	if path == "" {
		return modelConfig(loadDefaultIn(dir))
	}

	file := path
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}

	defaultFile, err := resolveDefaultConfig(dir)
	if err != nil || filepath.Clean(file) != defaultFile {
		return modelConfig(loadFromFile(file))
	}

	return modelConfig(loadDefaultIn(dir))
}

// modelConfig builds the config result of a read, keeping the API token of a config that fails validation or decoding
func modelConfig(cfg *Config, topology *model.Topology, source files, err error) model.Config {
	result := model.Config{Path: source.path, OverridePath: source.override, Error: err}

	switch {
	case err == nil:
		result.Project = Project(cfg, topology)
		result.Topology = *topology
	case cfg != nil:
		result.Project.Server.Token = cfg.Server.Auth.Token
	}

	return result
}
