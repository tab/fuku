package config

// LoadPath loads configuration from path, or from the default config file when path is empty
func LoadPath(path string) (*Config, *Topology, error) {
	if path != "" {
		return LoadFromFile(path)
	}

	return Load()
}

// ResolveConfigPath returns explicit when set, otherwise the first existing default config file
func ResolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	return resolveDefaultConfig()
}

// ResolveOverridePath returns the override file beside basePath, or empty when basePath is empty or no override exists
func ResolveOverridePath(basePath string) (string, error) {
	if basePath == "" {
		return "", nil
	}

	return resolveOverrideFile(basePath)
}
