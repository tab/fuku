package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

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
	Wait(ctx context.Context, id string) error
}

// Registry exposes the runtime read model the handlers answer from
type Registry interface {
	Read(fn func(*model.Snapshot))
	WaitSettled(ctx context.Context, id string, since time.Time) error
}

// maxWaitSeconds bounds how long an action request may hold its connection
const maxWaitSeconds = 60

// maxActionBody bounds the body of an action request
const maxActionBody = 1024

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

// handleAction admits one action and answers 202 with the predicted status, or 200 once the service settles
func (h *handler) handleAction(w http.ResponseWriter, r *http.Request, action contracts.Action, admit func(string) (services.Admission, error)) {
	wait, err := parseWait(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	id := r.PathValue("id")

	admission, err := admit(id)
	if err != nil {
		writeActionError(w, action, err)

		return
	}

	if wait == 0 || !h.settle(r.Context(), id, wait, admission.At) {
		writeJSON(w, http.StatusAccepted, toActionSerializer(admission))

		return
	}

	h.handleGetService(w, r)
}

// settle reports whether the admitted action ended and the read model shows its outcome within the wait
func (h *handler) settle(ctx context.Context, id string, wait time.Duration, since time.Time) bool {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	// ponytail: an unrequested exit the watcher reports right after admission ends the wait early; require a transitional status at or after since to close it
	if err := h.control.Wait(ctx, id); err != nil {
		return false
	}

	// sync: the read model applies the action's last event after the token is released, so wait for it too
	return h.registry.WaitSettled(ctx, id, since) == nil
}

// parseWait reads the optional wait of an action from its JSON body, in whole seconds from 1 up to maxWaitSeconds
func parseWait(w http.ResponseWriter, r *http.Request) (time.Duration, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxActionBody))
	if err != nil {
		return 0, ErrAPIInvalidBody
	}

	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, nil
	}

	var request ActionRequest

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil || !errors.Is(decoder.Decode(&struct{}{}), io.EOF) {
		return 0, ErrAPIInvalidBody
	}

	if request.Wait == nil {
		return 0, nil
	}

	if *request.Wait < 1 || *request.Wait > maxWaitSeconds {
		return 0, ErrAPIInvalidWait
	}

	return time.Duration(*request.Wait) * time.Second, nil
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
