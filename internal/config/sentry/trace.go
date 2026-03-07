package sentry

import (
	gosentry "github.com/getsentry/sentry-go"
)

// Re-exported Span API types from sentry-go
type (
	Span       = gosentry.Span
	SpanOption = gosentry.SpanOption
	SpanStatus = gosentry.SpanStatus
)

// Re-exported Span API functions from sentry-go
var (
	StartTransaction      = gosentry.StartTransaction
	WithTransactionSource = gosentry.WithTransactionSource
	WithDescription       = gosentry.WithDescription
)

// SourceTask is the re-exported TransactionSource constant from sentry-go
const SourceTask = gosentry.SourceTask

// Re-exported SpanStatus constants from sentry-go
const (
	SpanStatusOK       = gosentry.SpanStatusOK
	SpanStatusCanceled = gosentry.SpanStatusCanceled
)

// Span operation names for trace instrumentation
const (
	OpDiscovery      = "discovery"
	OpPreflight      = "preflight"
	OpTierStartup    = "tier_startup"
	OpShutdown       = "shutdown"
	OpWatchRestart   = "watch_restart"
	OpServiceStop    = "service_stop"
	OpServiceRestart = "service_restart"
)
