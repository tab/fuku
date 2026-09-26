package model

// Env is a single environment variable parsed from a service's .env files
type Env struct {
	Key   string
	Value string
}
