package bus

import (
	"bytes"
	"fmt"

	"fuku/internal/config/logger"
)

// Formatter formats bus events as text
type Formatter struct {
	event logger.EventLogger
}

// NewFormatter creates a new bus event formatter
func NewFormatter(event logger.EventLogger) *Formatter {
	return &Formatter{
		event: event,
	}
}

// Format formats a bus event as text
func (f *Formatter) Format(msgType MessageType, data any) string {
	var buf bytes.Buffer

	l := f.event.NewLogger(&buf)
	e := l.Log()

	switch d := data.(type) {
	case CommandStarted:
		e.Str("command", d.Command).Str("profile", d.Profile).Bool("ui", d.UI)
	case ProfileResolved:
		e.Str("profile", d.Profile)
	case PhaseChanged:
		e.Str("phase", string(d.Phase)).Str("duration", d.Duration.String()).Int("services", d.ServiceCount)
	case PreflightStarted:
		e.Strs("services", d.Services)
	case PreflightKill:
		e.Str("service", d.Service).Int("pid", d.PID).Str("name", d.Name)
	case PreflightComplete:
		e.Int("killed", d.Killed).Str("duration", d.Duration.String())
	case TierStarting:
		e.Str("tier", d.Name)
	case Service:
		e.Str("id", d.ID).Str("name", d.Name)
	case TierReady:
		e.Str("tier", d.Name).Str("duration", d.Duration.String()).Int("services", d.ServiceCount)
	case ServiceStarting:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier).Int("pid", d.PID)
	case ReadinessComplete:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("type", d.Type).Str("duration", d.Duration.String())
	case ServiceReady:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier)
	case ServiceFailed:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier)

		if d.Error != nil {
			e.Str("error", d.Error.Error())
		}
	case ServiceStopping:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier)
	case ServiceStopped:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier)
	case ServiceRestarting:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Str("tier", d.Tier)
	case Signal:
		e.Str("signal", d.Name)
	case WatchTriggered:
		e.Str("id", d.Service.ID).Str("service", d.Service.Name).Strs("files", d.ChangedFiles)
	case ResourceSample:
		e.Str("cpu", fmt.Sprintf("%.1f%%", d.CPU)).Str("mem", fmt.Sprintf("%.1fMB", d.MEM))
	case APIStarted:
		e.Str("listen", d.Listen)
	case APIStopped:
	case APIRequest:
		e.Str("method", d.Method).Str("path", d.Path).Int("status", d.Status).Str("duration", d.Duration.String())
	default:
		e.Interface("data", data)
	}

	e.Msg(string(msgType))

	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}
