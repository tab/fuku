package eventlog

import (
	"bytes"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Logfmt field keys
const (
	FieldCommand  = "command"
	FieldProfile  = "profile"
	FieldUI       = "ui"
	FieldPhase    = "phase"
	FieldDuration = "duration"
	FieldServices = "services"
	FieldService  = "service"
	FieldPID      = "pid"
	FieldAttempt  = "attempt"
	FieldName     = "name"
	FieldKilled   = "killed"
	FieldTier     = "tier"
	FieldID       = "id"
	FieldType     = "type"
	FieldError    = "error"
	FieldSignal   = "signal"
	FieldFiles    = "files"
	FieldCPU      = "cpu"
	FieldMem      = "mem"
	FieldListen   = "listen"
	FieldMethod   = "method"
	FieldPath     = "path"
	FieldStatus   = "status"
	FieldVersion  = "version"
	FieldData     = "data"
)

// Formatter formats bus events as text
type Formatter struct{}

// NewFormatter creates a new bus event formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Format formats a bus event as text
func (f *Formatter) Format(msgType contracts.MessageType, data any) string {
	var buf bytes.Buffer

	l := newEventLogger(&buf)
	e := l.Log()

	switch d := data.(type) {
	case contracts.CommandStarted:
		e.Str(FieldCommand, d.Command).Str(FieldProfile, d.Profile).Bool(FieldUI, d.UI)
	case contracts.ProfileResolved:
		e.Str(FieldProfile, d.Profile)
	case contracts.PhaseChanged:
		e.Str(FieldPhase, string(d.Phase)).Str(FieldDuration, d.Duration.String()).Int(FieldServices, d.ServiceCount)
	case contracts.PreflightStarted:
		e.Strs(FieldServices, d.Services)
	case contracts.PreflightKilled:
		e.Str(FieldService, d.Service).Int(FieldPID, d.PID).Str(FieldName, d.Name)
	case contracts.PreflightComplete:
		e.Int(FieldKilled, d.Killed).Str(FieldDuration, d.Duration.String())
	case contracts.TierStarting:
		e.Str(FieldTier, d.Name)
	case model.Service:
		e.Str(FieldID, d.ID).Str(FieldName, d.Name)
	case contracts.TierReady:
		e.Str(FieldTier, d.Name).Str(FieldDuration, d.Duration.String()).Int(FieldServices, d.ServiceCount)
	case contracts.ServiceStarting:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier).Int(FieldPID, d.PID).Int(FieldAttempt, d.Attempt)
	case contracts.ReadinessComplete:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldType, string(d.Type)).Str(FieldDuration, d.Duration.String())
	case contracts.ServiceReady:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier)
	case contracts.ServiceFailed:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier)

		if d.Error != nil {
			e.Str(FieldError, d.Error.Error())
		}
	case contracts.ServiceStopping:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier)
	case contracts.ServiceStopped:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier)
	case contracts.ServiceRestarting:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Str(FieldTier, d.Tier)
	case contracts.SignalReceived:
		e.Str(FieldSignal, d.Name)
	case contracts.WatchTriggered:
		e.Str(FieldID, d.Service.ID).Str(FieldService, d.Service.Name).Strs(FieldFiles, d.ChangedFiles)
	case contracts.WatchStarted:
		e.Str(FieldID, d.Service.ID).Str(FieldName, d.Service.Name)
	case contracts.WatchStopped:
		e.Str(FieldID, d.Service.ID).Str(FieldName, d.Service.Name)
	case contracts.ResourceSampled:
		e.Str(FieldCPU, fmt.Sprintf("%.1f%%", d.CPU)).Str(FieldMem, fmt.Sprintf("%.1fMB", float64(d.Memory)/1024/1024))
	case contracts.APIStarted:
		e.Str(FieldListen, d.Listen)
	case contracts.APIStopped:
	case contracts.APIRequested:
		e.Str(FieldMethod, d.Method).Str(FieldPath, d.Path).Int(FieldStatus, d.Status).Str(FieldDuration, d.Duration.String())
	case contracts.UpdateAvailable:
		e.Str(FieldVersion, d.Version)
	case nil:
	default:
		e.Interface(FieldData, data)
	}

	e.Msg(string(msgType))

	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}
