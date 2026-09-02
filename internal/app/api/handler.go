package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/app/registry"
	"fuku/internal/app/relay"
	"fuku/internal/config"
)

type handler struct {
	bus      bus.Bus
	store    registry.Store
	journal  Journal
	identity instance.Identity
}

// Journal reads the buffered service output kept by the log relay
type Journal interface {
	History(query relay.HistoryQuery) []relay.LogMessage
}

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
// (Revision changes on every lifecycle transition, so a client can tell an action it just
// requested from a state it observed earlier without comparing timestamps)
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
	Revision uint64          `json:"revision"`
}

// ServiceListSerializer serializes a list of services
type ServiceListSerializer struct {
	Services []ServiceSerializer `json:"services"`
}

// Action values reported in accepted action responses
const (
	actionStart   = "start"
	actionStop    = "stop"
	actionRestart = "restart"
)

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

// ProbeSerializer serializes a health probe response
type ProbeSerializer struct {
	Status string `json:"status"`
}

// LiveSerializer serializes the liveness probe, identifying the instance and the project it serves
// (the project is reported as a fingerprint because this endpoint is unauthenticated)
type LiveSerializer struct {
	Status   string `json:"status"`
	Product  string `json:"product"`
	Instance string `json:"instance"`
	Project  string `json:"project"`
}

// LogLineSerializer serializes one buffered log line
type LogLineSerializer struct {
	Service   string `json:"service"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// LogListSerializer serializes the buffered log lines matching a query
type LogListSerializer struct {
	Lines []LogLineSerializer `json:"lines"`
	Tail  int                 `json:"tail"`
}

func (h *handler) handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(LiveSerializer{
		Status:   "alive",
		Product:  config.AppName,
		Instance: h.identity.ID,
		Project:  h.identity.Fingerprint,
	})
}

func (h *handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !h.store.IsResolved() {
		w.WriteHeader(http.StatusServiceUnavailable)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ProbeSerializer{Status: "not ready"})

		return
	}

	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ProbeSerializer{Status: "ready"})
}

func (h *handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	c := h.store.Counts()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(StatusSerializer{
		Version:  config.Version,
		Instance: h.identity.ID,
		Project:  h.identity.Project,
		Profile:  h.store.Profile(),
		Phase:    h.store.Phase(),
		Uptime:   int64(h.store.Uptime().Seconds()),
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

	svc, found := h.store.Service(id)
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

	svc, found := h.store.Service(id)
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
		Data:     bus.Service{ID: svc.ID, Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: actionStart,
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

	svc, found := h.store.Service(id)
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
		Data:     bus.Service{ID: svc.ID, Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: actionStop,
		Status: registry.StatusStopping,
	})
}

//nolint:dupl // restart and start handlers share validation but differ in state check and response
func (h *handler) handleRestartService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if h.store.Phase() != string(bus.PhaseRunning) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotAccepting.Error()})

		return
	}

	svc, found := h.store.Service(id)
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	if !svc.Status.IsRestartable() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPINotRestartable.Error()})

		return
	}

	h.bus.Publish(bus.Message{
		Type:     bus.CommandRestartService,
		Data:     bus.Service{ID: svc.ID, Name: svc.Name},
		Critical: true,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(ActionSerializer{
		ID:     svc.ID,
		Name:   svc.Name,
		Action: actionRestart,
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
		Revision: s.LifecycleSeq,
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

// parseLogQuery reads the bounded log query parameters shared by both log endpoints
func parseLogQuery(values url.Values) (relay.HistoryQuery, error) {
	query := relay.HistoryQuery{Tail: config.APILogsDefaultTail}

	if raw := values.Get("tail"); raw != "" {
		tail, err := strconv.Atoi(raw)
		if err != nil || tail <= 0 {
			return query, errors.ErrAPIInvalidTail
		}

		query.Tail = min(tail, config.SocketLogsHistorySize)
	}

	if raw := values.Get("since"); raw != "" {
		since, err := time.ParseDuration(raw)
		if err != nil || since <= 0 {
			return query, errors.ErrAPIInvalidSince
		}

		query.Since = time.Now().Add(-since)
	}

	return query, nil
}

// writeLogs renders the buffered lines matching the query
func (h *handler) writeLogs(w http.ResponseWriter, r *http.Request, services []string) {
	query, err := parseLogQuery(r.URL.Query())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: err.Error()})

		return
	}

	query.Services = services

	messages := h.journal.History(query)
	lines := make([]LogLineSerializer, len(messages))

	for i, msg := range messages {
		lines[i] = LogLineSerializer{
			Service:   msg.Service,
			Message:   msg.Message,
			Timestamp: msg.Timestamp.UTC().Format(time.RFC3339Nano),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(LogListSerializer{Lines: lines, Tail: query.Tail})
}

func (h *handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	ids := r.URL.Query()["service"]
	if len(ids) == 0 {
		h.writeLogs(w, r, nil)

		return
	}

	names := make([]string, 0, len(ids))

	for _, id := range ids {
		svc, found := h.store.Service(id)
		if !found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

			return
		}

		names = append(names, svc.Name)
	}

	h.writeLogs(w, r, names)
}

func (h *handler) handleServiceLogs(w http.ResponseWriter, r *http.Request) {
	svc, found := h.store.Service(r.PathValue("id"))
	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		//nolint:errcheck // best-effort JSON encoding
		json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIServiceNotFound.Error()})

		return
	}

	h.writeLogs(w, r, []string{svc.Name})
}
