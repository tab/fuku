package config

import (
	"fmt"
	"io"
	"os"

	"fuku/internal/adapters/config/template"
)

// Create writes the embedded fuku.yaml template in the current directory
func Create(stdout io.Writer) (int, error) {
	for _, file := range []string{ConfigFile, ConfigFileAlt} {
		_, err := os.Stat(file)

		switch {
		case err == nil:
			fmt.Fprintf(stdout, "%s already exists\n", file)
			return 0, nil
		case os.IsNotExist(err):
			continue
		default:
			return 1, fmt.Errorf("failed to check %s: %w", file, err)
		}
	}

	file, err := os.OpenFile(ConfigFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", ConfigFile, err)
	}

	defer file.Close()

	if _, err := file.Write(template.Content); err != nil {
		return 1, fmt.Errorf("failed to write %s: %w", ConfigFile, err)
	}

	fmt.Fprintf(stdout, "Created %s\n", ConfigFile)

	return 0, nil
}
