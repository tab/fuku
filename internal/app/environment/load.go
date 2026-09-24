package environment

import "fuku/internal/model"

// Default .env file names merged when a service does not override env.files
const (
	fileEnv                 = ".env"
	fileEnvLocal            = ".env.local"
	fileEnvDevelopment      = ".env.development"
	fileEnvDevelopmentLocal = ".env.development.local"
)

// reload replaces the cached entries of a service with a fresh read of its .env files (a nil service clears them)
func (s *Store) reload(id string, service *model.Service) {
	entries := s.load(service)

	s.mu.Lock()
	s.cache[id] = entries
	s.mu.Unlock()
}

// load reads and merges the .env files the service selects
func (s *Store) load(service *model.Service) []model.Env {
	if service == nil || service.Directory == "" {
		return nil
	}

	return s.merge(service.Directory, resolveFiles(service))
}

// resolveFiles returns the configured env.files list, falling back to defaults when unset
func resolveFiles(service *model.Service) []string {
	if service.Environment != nil && service.Environment.Files != nil {
		return service.Environment.Files
	}

	return []string{fileEnv, fileEnvLocal, fileEnvDevelopment, fileEnvDevelopmentLocal}
}
