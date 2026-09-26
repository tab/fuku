package services

import "errors"

// errStopAll is the cancellation cause of a StopAll command (a clean end of the run, like a cancelled context)
var errStopAll = errors.New("StopAll command")

// errStartupInterrupted marks a run cut short during startup, wrapped with the context's cancellation cause
var errStartupInterrupted = errors.New("startup interrupted")
