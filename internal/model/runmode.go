package model

// RunMode is how a run holds its terminal: under the TUI, in the foreground without it, or detached in the background
type RunMode string

// RunMode values
const (
	RunModeTUI        RunMode = "tui"
	RunModeForeground RunMode = "foreground"
	RunModeDetached   RunMode = "detached"
)
