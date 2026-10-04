package detach

import "time"

// Options are the settings of the detached run the parent passes to its child
type Options struct {
	Profile    string
	ConfigFile string
}

// StopOptions bound the wait for the running instance to exit before it is killed
type StopOptions struct {
	Timeout time.Duration
}
