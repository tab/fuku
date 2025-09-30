package readiness

import (
	"context"
	"fmt"
	"time"

	"fuku/internal/config"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 500 * time.Millisecond
)

// Checker defines the interface for readiness checking
//
//go:generate mockgen -source=readiness.go -destination=readiness_mock.go -package=readiness
type Checker interface {
	Check(ctx context.Context) error
}

// Factory creates readiness checkers based on configuration
type Factory interface {
	CreateChecker(check *config.ReadinessCheck) (Checker, error)
}

// factory implements Factory interface
type factory struct{}

// NewFactory creates a new readiness checker factory
func NewFactory() Factory {
	return &factory{}
}

// CreateChecker creates a readiness checker based on configuration
func (f *factory) CreateChecker(check *config.ReadinessCheck) (Checker, error) {
	if check == nil {
		return nil, nil
	}

	timeout := defaultTimeout
	if check.Timeout != "" {
		if d, err := time.ParseDuration(check.Timeout); err == nil {
			timeout = d
		}
	}

	interval := defaultInterval
	if check.Interval != "" {
		if d, err := time.ParseDuration(check.Interval); err == nil {
			interval = d
		}
	}

	switch check.Type {
	case "http":
		if check.URL == "" {
			return nil, fmt.Errorf("http readiness check requires url")
		}
		return NewHTTPChecker(check.URL, timeout, interval), nil

	case "log":
		if check.Pattern == "" {
			return nil, fmt.Errorf("log readiness check requires pattern")
		}
		return NewLogChecker(check.Pattern, timeout)

	default:
		return nil, fmt.Errorf("unsupported readiness check type: %s", check.Type)
	}
}
