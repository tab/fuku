package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"fuku/internal/app/services"
	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// Control admits the service actions the handlers accept
type Control interface {
	Start(id string) (services.Admission, error)
	Stop(id string) (services.Admission, error)
	Restart(id string) (services.Admission, error)
}

// Registry exposes the runtime read model the handlers answer from
type Registry interface {
	Read(fn func(*model.Snapshot))
}

type handler struct {
	registry Registry
	control  Control
	identity model.Instance
}

// conflicts is the 409 text of a rejected action
var conflicts = map[contracts.Action]error{
	contracts.ActionStart:   ErrAPINotStartable,
	contracts.ActionStop:    ErrAPINotRunning,
	contracts.ActionRestart: ErrAPINotRestartable,
}

func (h *handler) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, LiveSerializer{
		Status:      "alive",
		Product:     buildinfo.AppName,
		Instance:    h.identity.ID,
		Fingerprint: h.identity.Fingerprint,
	})
}

func (h *handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	var resolved bool

	h.registry.Read(func(snapshot *model.Snapshot) {
		resolved = snapshot.Resolved
	})

	if !resolved {
		writeJSON(w, http.StatusServiceUnavailable, ProbeSerializer{Status: "not ready"})

		return
	}

	writeJSON(w, http.StatusOK, ProbeSerializer{Status: "ready"})
}

func (h *handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	var status StatusSerializer

	h.registry.Read(func(snapshot *model.Snapshot) {
		status = toStatusSerializer(snapshot, h.identity, buildinfo.Version)
	})

	writeJSON(w, http.StatusOK, status)
}

func (h *handler) handleListServices(w http.ResponseWriter, _ *http.Request) {
	var list ServiceListSerializer

	h.registry.Read(func(snapshot *model.Snapshot) {
		list = toServiceListSerializer(snapshot.Tiers)
	})

	writeJSON(w, http.StatusOK, list)
}

func (h *handler) handleGetService(w http.ResponseWriter, r *http.Request) {
	var (
		service ServiceSerializer
		found   bool
	)

	h.registry.Read(func(snapshot *model.Snapshot) {
		svc, exists := snapshot.Services[r.PathValue("id")]
		if exists {
			service, found = toServiceSerializer(svc), true
		}
	})

	if !found {
		writeError(w, http.StatusNotFound, ErrAPIServiceNotFound)

		return
	}

	writeJSON(w, http.StatusOK, service)
}

func (h *handler) handleStartService(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, contracts.ActionStart, h.control.Start)
}

func (h *handler) handleStopService(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, contracts.ActionStop, h.control.Stop)
}

func (h *handler) handleRestartService(w http.ResponseWriter, r *http.Request) {
	h.handleAction(w, r, contracts.ActionRestart, h.control.Restart)
}

// handleAction admits one action through the control and answers 202 with the predicted status
func (h *handler) handleAction(w http.ResponseWriter, r *http.Request, action contracts.Action, admit func(string) (services.Admission, error)) {
	admission, err := admit(r.PathValue("id"))
	if err != nil {
		writeActionError(w, action, err)

		return
	}

	writeJSON(w, http.StatusAccepted, toActionSerializer(admission))
}

// writeActionError maps a rejected admission onto the API's status codes and texts
func writeActionError(w http.ResponseWriter, action contracts.Action, err error) {
	switch {
	case errors.Is(err, contracts.ErrServiceNotFound):
		writeError(w, http.StatusNotFound, ErrAPIServiceNotFound)
	case errors.Is(err, contracts.ErrNotAccepting), errors.Is(err, contracts.ErrBusClosed):
		writeError(w, http.StatusConflict, ErrAPINotAccepting)
	case errors.Is(err, contracts.ErrActionNotAllowed), errors.Is(err, contracts.ErrServiceBusy):
		writeError(w, http.StatusConflict, conflicts[action])
	default:
		writeError(w, http.StatusInternalServerError, ErrAPIOverloaded)
	}
}

// writeError answers with the error envelope
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, ErrorSerializer{Error: err.Error()})
}

// writeJSON answers with a JSON body
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	//nolint:errcheck // best-effort JSON encoding
	json.NewEncoder(w).Encode(body)
}
