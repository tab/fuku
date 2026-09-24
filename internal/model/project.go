package model

import "time"

// ProfileDefault is the profile run when the command names none
const ProfileDefault = "default"

// Project is the loaded configuration as plain values: the service catalog, the profiles and the run-wide settings
type Project struct {
	Services    []Service
	Profiles    map[string]Profile
	Exclude     []string
	Logging     Logging
	Concurrency Concurrency
	Retry       Retry
	Logs        Logs
	Server      Server
	Telemetry   Telemetry
	Updater     Updater
}

// Service returns the configuration of the named service and whether the project declares it
func (p Project) Service(name string) (Service, bool) {
	for _, service := range p.Services {
		if service.Name == name {
			return service, true
		}
	}

	return Service{}, false
}

// Profile selects all services or a named service list
type Profile struct {
	All      bool
	Services []string
}

// ReadinessType selects the probe a service is checked with
type ReadinessType string

// Readiness probe types
const (
	ReadinessHTTP ReadinessType = "http"
	ReadinessTCP  ReadinessType = "tcp"
	ReadinessLog  ReadinessType = "log"
)

// Readiness describes one service readiness probe
type Readiness struct {
	Type     ReadinessType
	Address  string
	URL      string
	Pattern  string
	Timeout  time.Duration
	Interval time.Duration
}

// Watch describes the paths and delay used to restart a changed service
type Watch struct {
	Include  []string
	Ignore   []string
	Shared   []string
	Debounce time.Duration
}

// EnvFiles lists the environment files shown by fuku (an empty list disables loading, nil means the defaults)
type EnvFiles struct {
	Files []string
}

// Logging is the application log level and output format
type Logging struct {
	Level  string
	Format string
}

// Concurrency bounds the services started at once
type Concurrency struct {
	Workers int
}

// Retry is the service start retry policy
type Retry struct {
	Attempts int
	Backoff  time.Duration
}

// Logs sizes the per-client log queue and the replay history
type Logs struct {
	Buffer  int
	History int
}

// Server is the REST API bind address and its bearer token (an empty Listen disables the server)
type Server struct {
	Listen string
	Token  string
}

// Telemetry is the telemetry opt-in with the DSN and environment read from the process environment
type Telemetry struct {
	Enabled     bool
	DSN         string
	Environment string
}

// Updater is the release check opt-in
type Updater struct {
	Enabled bool
}
