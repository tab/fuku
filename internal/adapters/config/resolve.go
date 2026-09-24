package config

import "fuku/internal/model"

// LoadPath loads the config at path (the default file with its override when empty) and carries a failure in Error
func LoadPath(path string) model.Config {
	cfg, topology, source, err := load(path)

	result := model.Config{Path: source.path, OverridePath: source.override}

	if err != nil {
		result.Error = err

		return result
	}

	result.Project = Project(cfg, topology)
	result.Topology = *topology

	return result
}

// load reads an explicit path without override merging, or the default file with its override when path is empty
func load(path string) (*Config, *model.Topology, files, error) {
	if path != "" {
		return loadFromFile(path)
	}

	return loadDefault()
}
