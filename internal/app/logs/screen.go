package logs

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"

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
	client relay.Client
	log    logger.Logger
	render *render.Log
	format string
	out    io.Writer
	width  func() int
}

// NewScreen creates a new logs screen
func NewScreen(client relay.Client, log logger.Logger, r *render.Log, cfg *config.Config) Screen {
	return &screen{
		client: client,
		log:    log.WithComponent("LOGS"),
		render: r,
		format: cfg.Logging.Format,
		out:    os.Stdout,
		width:  terminalWidth,
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
	socketPath, err := relay.FindSocket(config.SocketDir, options.Profile)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to find socket")
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
		subscribed: options.Services,
		out:        s.out,
		width:      s.width,
		noUI:       options.NoUI,
	}

	if err := s.client.Stream(ctx, handler); err != nil {
		s.log.Error().Err(err).Msg("Failed to stream logs")
		return 1
	}

	return 0
}

// screenHandler implements relay.Handler for the logs screen
type screenHandler struct {
	render     *render.Log
	format     string
	subscribed []string
	out        io.Writer
	width      func() int
	noUI       bool
}

// HandleStatus renders the connection banner unless the UI is disabled
func (h *screenHandler) HandleStatus(status relay.StatusMessage) {
	if h.noUI {
		return
	}

	h.render.RenderBanner(h.out, h.width(), status, h.subscribed)
}

// HandleLog writes a formatted log line
func (h *screenHandler) HandleLog(msg relay.LogMessage) {
	line := h.render.FormatMessage(h.format, msg.Service, msg.Message)
	//nolint:errcheck // best-effort write to output
	io.WriteString(h.out, line)
}
