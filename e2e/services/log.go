package services

import "context"

// LogService is a service with log-based readiness
type LogService struct {
	Service
}

// NewLog creates a log-based readiness service
func NewLog(name string) Runner {
	return &LogService{Service: newService(name)}
}

// Run starts the service and waits for shutdown
func (s *LogService) Run() {
	s.Log.Info().Msg("Starting service")
	s.Log.Info().Msg("Service ready")

	s.WaitForShutdown(context.Background())

	s.Log.Info().Msg("Shutting down")
}
