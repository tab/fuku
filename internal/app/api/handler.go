package api

import (
	"encoding/json"
	"net/http"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/registry"
	"fuku/internal/config"
)

type handler struct {
	store registry.Store
	bus   bus.Bus
}

// StatusSerializer serializes the fuku instance status
type StatusSerializer struct {
	Version  string                 `json:"version"`
	Profile  string                 `json:"profile"`
	Phase    string                 `json:"phase"`
	Uptime   int64                  `json:"uptime"`
	Services ServiceCountSerializer `json:"services"`
}

// ServiceCountSerializer serializes service counts by status
type ServiceCountSerializer struct {
	Total      int `json:"total"`
	Starting   int `json:"starting"`
	Running    int `json:"running"`
	Stopping   int `json:"stopping"`
	Restarting int `json:"restarting"`
	Stopped    int `json:"stopped"`
	Failed     int `json:"failed"`
}

// ServiceSerializer serializes a single service
type ServiceSerializer struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Tier     string          `json:"tier"`
	Status   registry.Status `json:"status"`
	Watching bool            `json:"watching"`
	Error    string          `json:"error,omitempty"`
	PID      int             `json:"pid"`
	CPU      float64         `json:"cpu"`
	Memory   uint64          `json:"memory"`
	Uptime   int64           `json:"uptime"`
}

// ServiceListSerializer serializes a list of services
type ServiceListSerializer struct {
	Services []ServiceSerializer `json:"services"`
}

// ActionSerializer serializes an accepted action response
type ActionSerializer struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Action string          `json:"action"`
	Status registry.Status `json:"status"`
}

// ErrorSerializer serializes an error response
type ErrorSerializer struct {
	Error string `json:"error"`
}

func (h *handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	services := h.store.Services()
	counts := ServiceCountSerializer{Total: len(services)}

	for _, svc := range services {
		switch svc.Status {
		case registry.StatusStarting:
			counts.Starting++
		case registry.StatusRunning:
			counts.Running++
		case registry.StatusStopping:
			counts.Stopping++
		case registry.StatusRestarting:
			counts.Restarting++
		case registry.StatusStopped:
			counts.Stopped++
		case registry.StatusFailed:
			counts.Failed++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(StatusSerializer{
		Version:  config.Version,
		Profile:  h.store.Profile(),
		Phase:    h.store.Phase(),
		Uptime:   int64(h.store.Uptime().Seconds()),
		Services: counts,
	})
}

func (h *handler) handleListServices(w http.ResponseWriter, _ *http.Request) {
	snapshots := h.store.Services()
	services := make([]ServiceSerializer, len(snapshots))

	for i, s := range snapshots {
		services[i] = toServiceSerializer(s)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ServiceListSerializer{Services: services})
}

func (h *handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	svc, found := h.store.ServiceByID(id)
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(toServiceSerializer(svc))
}

//nolint:dupl // start, stop and restart handlers share validation but differ in command and response
func (h *handler) handleStartService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.store.Phase() != string(bus.PhaseRunning) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotAccepting.Error()})

		return
	}

	svc, found := h.store.ServiceByID(id)
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	if !svc.Status.IsStartable() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotStartable.Error()})

		return
	}

	h.bus.Publish(bus.Message{
		Type:     bus.CommandStartService,
		Data:     bus.Payload{Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: "start",
		Status: registry.StatusStarting,
	})
}

//nolint:dupl // stop and restart handlers share validation but differ in command and response
func (h *handler) handleStopService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.store.Phase() != string(bus.PhaseRunning) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotAccepting.Error()})

		return
	}

	svc, found := h.store.ServiceByID(id)
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	if !svc.Status.IsStoppable() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotRunning.Error()})

		return
	}

	h.bus.Publish(bus.Message{
		Type:     bus.CommandStopService,
		Data:     bus.Payload{Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: "stop",
		Status: registry.StatusStopping,
	})
}

//nolint:dupl // restart and stop handlers share validation but differ in command and response
func (h *handler) handleRestartService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.store.Phase() != string(bus.PhaseRunning) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotAccepting.Error()})

		return
	}

	svc, found := h.store.ServiceByID(id)
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	if !svc.Status.IsStoppable() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotRunning.Error()})

		return
	}

	h.bus.Publish(bus.Message{
		Type:     bus.CommandRestartService,
		Data:     bus.Payload{Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: "restart",
		Status: registry.StatusRestarting,
	})
}

func toServiceSerializer(s registry.ServiceSnapshot) ServiceSerializer {
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

	result.PID = s.PID
	result.CPU = s.CPU
	result.Memory = s.Memory

	if !s.StartTime.IsZero() {
		result.Uptime = int64(time.Since(s.StartTime).Seconds())
	}

	return result
}
