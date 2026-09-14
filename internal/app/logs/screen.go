package logs

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"

	"fuku/internal/app/errors"
	"fuku/internal/app/instance"
	"fuku/internal/app/relay"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Options describes one log stream request
type Options struct {
	Profile  string
	Services []string
	NoUI     bool
	relay.ReplayOptions
}

// Screen handles the fuku logs command
type Screen interface {
	Run(ctx context.Context, options Options) int
}

// screen implements the Screen interface
type screen struct {
	client      relay.Client
	render      *render.Log
	format      string
	project     string
	fingerprint string
	out         io.Writer
	width       func() int
	log         logger.Logger
}

// NewScreen creates a new logs screen
func NewScreen(client relay.Client, r *render.Log, cfg *config.Config, identity instance.Identity, log logger.Logger) Screen {
	return &screen{
		client:      client,
		render:      r,
		format:      cfg.Logging.Format,
		project:     identity.Project,
		fingerprint: identity.Fingerprint,
		out:         os.Stdout,
		width:       terminalWidth,
		log:         log.WithComponent("LOGS"),
	}
}

// terminalWidth returns the current terminal width
func terminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w < 40 {
		return 80
	}

	return w
}

// Run handles the logs command to stream logs from a running instance
func (s *screen) Run(ctx context.Context, options Options) int {
	socketPath, err := relay.FindSocket(config.SocketDir, s.fingerprint)
	if err != nil {
		s.log.Error().Err(err).Msgf("No fuku is running for project '%s'", s.project)
		return 1
	}

	return s.streamLogs(ctx, socketPath, options)
}

// streamLogs connects to a running fuku instance and streams logs
func (s *screen) streamLogs(ctx context.Context, socketPath string, options Options) int {
	if err := s.client.Connect(socketPath); err != nil {
		s.log.Error().Err(err).Msg("Failed to connect to socket")
		return 1
	}

	defer s.client.Close()

	subscription := relay.SubscribeOptions{
		Services:      options.Services,
		ReplayOptions: options.ReplayOptions,
	}

	if err := s.client.Subscribe(subscription); err != nil {
		s.log.Error().Err(err).Msg("Failed to subscribe to services")
		return 1
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	handler := &screenHandler{
		render:     s.render,
		format:     s.format,
		profile:    options.Profile,
		subscribed: options.Services,
		out:        s.out,
		width:      s.width,
		noUI:       options.NoUI,
		log:        s.log,
	}

	err := s.client.Stream(ctx, handler)

	if errors.Is(err, errors.ErrProfileMismatch) {
		return 1
	}

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to stream logs")
		return 1
	}

	return 0
}

// screenHandler implements relay.Handler for the logs screen
type screenHandler struct {
	render     *render.Log
	format     string
	profile    string
	subscribed []string
	out        io.Writer
	width      func() int
	noUI       bool
	log        logger.Logger
}

// HandleStatus checks the expected profile and renders the connection banner unless the UI is disabled
func (h *screenHandler) HandleStatus(status relay.StatusMessage) error {
	if h.profile != "" && status.Profile != h.profile {
		h.log.Error().Msgf("Fuku is running profile '%s', not '%s'", status.Profile, h.profile)

		return errors.ErrProfileMismatch
	}

	if h.noUI {
		return nil
	}

	h.render.RenderBanner(h.out, h.width(), status, h.subscribed)

	return nil
}

// HandleLog writes a formatted log line
func (h *screenHandler) HandleLog(msg relay.LogMessage) {
	line := h.render.FormatMessage(h.format, msg.Service, msg.Message)
	//nolint:errcheck // best-effort write to output
	io.WriteString(h.out, line)
}
