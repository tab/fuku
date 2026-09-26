package model

// Config is the outcome of loading the configuration (Error is a failure that leaves Project and Topology empty)
type Config struct {
	Path         string
	OverridePath string
	Project      Project
	Topology     Topology
	Error        error
}
