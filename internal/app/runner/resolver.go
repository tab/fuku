package runner

import (
	"fmt"

	"fuku/internal/config"
)

func resolveServiceOrder(cfg *config.Config, serviceNames []string) ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	result := make([]string, 0, len(serviceNames))

	var visit func(string) error
	visit = func(serviceName string) error {
		if visiting[serviceName] {
			return fmt.Errorf("circular dependency detected for service '%s'", serviceName)
		}
		if visited[serviceName] {
			return nil
		}

		visiting[serviceName] = true

		service, exists := cfg.Services[serviceName]
		if !exists {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		for _, dep := range service.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[serviceName] = false
		visited[serviceName] = true
		result = append(result, serviceName)

		return nil
	}

	for _, serviceName := range serviceNames {
		if err := visit(serviceName); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func groupByDependencyLevel(services []string, cfg *config.Config) [][]string {
	levels := make(map[string]int)

	var calculateLevel func(string) int
	calculateLevel = func(name string) int {
		if level, ok := levels[name]; ok {
			return level
		}

		service, exists := cfg.Services[name]
		if !exists {
			levels[name] = 0
			return 0
		}

		maxDepLevel := -1
		for _, dep := range service.DependsOn {
			depLevel := calculateLevel(dep)
			if depLevel > maxDepLevel {
				maxDepLevel = depLevel
			}
		}

		levels[name] = maxDepLevel + 1
		return maxDepLevel + 1
	}

	for _, svc := range services {
		calculateLevel(svc)
	}

	maxLevel := 0
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}

	batches := make([][]string, maxLevel+1)
	for _, svc := range services {
		level := levels[svc]
		batches[level] = append(batches[level], svc)
	}

	return batches
}
