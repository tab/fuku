package environment

import "fuku/internal/model"

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

	return s.merge(service.Directory, service.Environment.Files)
}
