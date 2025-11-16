package navigation

// View represents the current view in the UI
type View int

const (
	ViewServices View = iota
	ViewLogs
)

// String returns the string representation of the view
func (v View) String() string {
	switch v {
	case ViewServices:
		return "services"
	case ViewLogs:
		return "logs"
	default:
		return "unknown"
	}
}
