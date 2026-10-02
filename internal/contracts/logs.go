package contracts

// LogStatus is what a running instance reports to a log client once it accepts the subscription
type LogStatus struct {
	Version  string
	Profile  string
	Services []string
	PID      int
}
