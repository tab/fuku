package model

// LogLine is one line a service wrote, as the log pipeline carries it from the process to the log clients
type LogLine struct {
	Service string
	Message string
}
