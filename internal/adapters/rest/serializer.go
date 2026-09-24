package rest

import (
	"time"

	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
)

// StatusSerializer serializes the fuku instance status
type StatusSerializer struct {
	Version  string                 `json:"version"`
	Instance string                 `json:"instance"`
	Project  string                 `json:"project"`
	Profile  string                 `json:"profile"`
	Phase    string                 `json:"phase"`
	Uptime   int64                  `json:"uptime"`
	Services ServiceCountSerializer `json:"services"`
}

// ServiceCountSerializer serializes service counts by status
type ServiceCountSerializer struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	Starting   int `json:"starting"`
	Running    int `json:"running"`
	Stopping   int `json:"stopping"`
	Restarting int `json:"restarting"`
	Stopped    int `json:"stopped"`
	Failed     int `json:"failed"`
}

// ServiceSerializer serializes a single service
type ServiceSerializer struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Tier     string       `json:"tier"`
	Status   model.Status `json:"status"`
	Watching bool         `json:"watching"`
	Error    string       `json:"error,omitempty"`
	PID      int          `json:"pid"`
	CPU      float64      `json:"cpu"`
	Memory   uint64       `json:"memory"`
	Uptime   int64        `json:"uptime"`
}

// ServiceListSerializer serializes a list of services
type ServiceListSerializer struct {
	Services []ServiceSerializer `json:"services"`
}

// ActionSerializer serializes an accepted action response
type ActionSerializer struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Action contracts.Action `json:"action"`
	Status model.Status     `json:"status"`
}

// ErrorSerializer serializes an error response
type ErrorSerializer struct {
	Error string `json:"error"`
}

// ProbeSerializer serializes a health probe response
type ProbeSerializer struct {
	Status string `json:"status"`
}

// LiveSerializer serializes the unauthenticated liveness probe, so the project appears as a fingerprint
type LiveSerializer struct {
	Status      string `json:"status"`
	Product     string `json:"product"`
	Instance    string `json:"instance"`
	Fingerprint string `json:"fingerprint"`
}

// toStatusSerializer serializes the instance status from one snapshot
func toStatusSerializer(snapshot *model.Snapshot, identity model.Instance, version string) StatusSerializer {
	c := snapshot.Counts()

	return StatusSerializer{
		Version:  version,
		Instance: identity.ID,
		Project:  identity.Project,
		Profile:  snapshot.Profile,
		Phase:    string(snapshot.Phase),
		Uptime:   uptime(snapshot.StartedAt),
		Services: ServiceCountSerializer{
			Total:      c.Total,
			Pending:    c.Pending,
			Starting:   c.Starting,
			Running:    c.Running,
			Stopping:   c.Stopping,
			Restarting: c.Restarting,
			Stopped:    c.Stopped,
			Failed:     c.Failed,
		},
	}
}

// toServiceListSerializer serializes the services in the order of the tiers
func toServiceListSerializer(tiers []*model.Tier) ServiceListSerializer {
	services := make([]ServiceSerializer, 0)

	for _, tier := range tiers {
		for _, s := range tier.Services {
			services = append(services, toServiceSerializer(s))
		}
	}

	return ServiceListSerializer{Services: services}
}

// toServiceSerializer serializes one service (process facts only while it runs)
func toServiceSerializer(s *model.Service) ServiceSerializer {
	result := ServiceSerializer{
		ID:       s.ID,
		Name:     s.Name,
		Tier:     s.Tier,
		Status:   s.Status,
		Watching: s.Watching,
		Error:    s.Error,
	}

	if !s.Status.IsRunning() {
		return result
	}

	result.PID = s.Process.PID
	result.CPU = s.Process.CPU
	result.Memory = s.Process.Memory
	result.Uptime = uptime(s.Process.StartedAt)

	return result
}

// toActionSerializer serializes an admitted action with the status it leads to
func toActionSerializer(admission services.Admission) ActionSerializer {
	return ActionSerializer{
		ID:     admission.Service.ID,
		Name:   admission.Service.Name,
		Action: admission.Action,
		Status: admission.Status,
	}
}

// uptime returns the whole seconds since start, or zero before it
func uptime(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}

	return int64(time.Since(start).Seconds())
}
