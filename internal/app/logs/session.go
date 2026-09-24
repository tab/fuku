package logs

import (
	"context"
	"errors"
	"fmt"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Request describes one log stream request
type Request struct {
	Profile  string
	Services []string
	model.ReplayOptions
}

// Client connects to the running instance of the project and streams its logs
type Client interface {
	Connect() error
	Subscribe(services []string, replay model.ReplayOptions) error
	Stream(ctx context.Context, handler Handler) error
	Close() error
}

// Handler receives the typed notifications of a log stream
type Handler interface {
	HandleStatus(status contracts.LogStatus) error
	HandleLog(line model.LogLine)
}

// View presents the accepted stream and its lines
type View interface {
	Status(status contracts.LogStatus, subscribed []string)
	Line(line model.LogLine)
}

// Session streams the logs of the running instance of the project
type Session struct {
	client  Client
	view    View
	project string
}

// NewSession creates the log session of the project the identity names
func NewSession(client Client, view View, identity model.Instance) *Session {
	return &Session{
		client:  client,
		view:    view,
		project: identity.Project,
	}
}

// Run streams the requested logs until the stream or ctx ends (a missing instance is returned with its project named)
func (s *Session) Run(ctx context.Context, request Request) error {
	if err := s.client.Connect(); err != nil {
		return s.connectError(err)
	}

	defer s.client.Close()

	if err := s.client.Subscribe(request.Services, request.ReplayOptions); err != nil {
		return err
	}

	return s.client.Stream(ctx, &stream{
		view:       s.view,
		profile:    request.Profile,
		subscribed: request.Services,
	})
}

// connectError names the project a missing instance was looked up for and passes any other connect failure as is
func (s *Session) connectError(err error) error {
	if errors.Is(err, contracts.ErrNoInstanceRunning) {
		return fmt.Errorf("%w for project '%s'", err, s.project)
	}

	return err
}

// stream checks the profile of one run and hands the notifications to the view
type stream struct {
	view       View
	profile    string
	subscribed []string
}

// HandleStatus rejects an instance running another profile than the requested one and presents the accepted stream
func (h *stream) HandleStatus(status contracts.LogStatus) error {
	if h.profile != "" && status.Profile != h.profile {
		return fmt.Errorf("%w: '%s' instead of '%s'", contracts.ErrProfileMismatch, status.Profile, h.profile)
	}

	h.view.Status(status, h.subscribed)

	return nil
}

// HandleLog presents one line
func (h *stream) HandleLog(line model.LogLine) {
	h.view.Line(line)
}
