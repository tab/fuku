package logging

// Level names accepted by the logging configuration
const (
	LevelTrace = "trace"
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
	LevelPanic = "panic"
)

// Output formats accepted by the logging configuration
const (
	FormatConsole = "console"
	FormatJSON    = "json"
)

// Options configures the application logger
type Options struct {
	Level   string
	Version string
}
