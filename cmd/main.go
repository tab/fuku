package main

import (
	"os"

	"fuku/internal/bootstrap"
)

// sentryDSN is the build-time Sentry DSN set through ldflags
var sentryDSN string

// main is the entry point for the application
func main() {
	exitCode := bootstrap.Run(os.Args[1:], sentryDSN)

	os.Exit(exitCode)
}
