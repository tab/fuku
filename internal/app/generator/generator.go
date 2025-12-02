package generator

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"text/template"

	"fuku/internal/config/logger"
)

const (
	templatePath = "templates/fuku.yaml.tmpl"
	fileName     = "fuku.yaml"
)

//go:embed templates/fuku.yaml.tmpl
var templateFS embed.FS

// Options contains the configuration for generating fuku.yaml
type Options struct {
	ProfileName string
	ServiceName string
}

// DefaultOptions returns sensible defaults for generation
func DefaultOptions() Options {
	return Options{
		ProfileName: "default",
		ServiceName: "api",
	}
}

// Generator defines the interface for generating fuku.yaml
type Generator interface {
	Generate(opts Options, force bool, dryRun bool) error
}

type generator struct {
	log logger.Logger
}

// NewGenerator creates a new generator instance
func NewGenerator(log logger.Logger) Generator {
	return &generator{
		log: log,
	}
}

// Generate creates a fuku.yaml file from the template
func (g *generator) Generate(opts Options, force bool, dryRun bool) error {
	if !dryRun && !force {
		if _, err := os.Stat(fileName); err == nil {
			return fmt.Errorf("file %s already exists, use --force to overwrite", fileName)
		}
	}

	tmplContent, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	tmpl, err := template.New("fuku.yaml").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, opts); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if dryRun {
		fmt.Print(buf.String())
		return nil
	}

	if err := os.WriteFile(fileName, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	g.log.Info().Msgf("Generated %s", fileName)

	return nil
}
