package logs

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"

	"fuku/internal/app/instance"
	"fuku/internal/app/relay"
	"fuku/internal/app/render"
	"fuku/internal/config"
	"fuku/internal/config/logger"
)

// Screen handles the fuku logs command
type Screen interface {
	Run(ctx context.Context, options StreamOptions) int
}

// StreamOptions selects the profile, services and history a log stream reads
type StreamOptions struct {
	Profile  string
	Services []string
	Tail     int
	Since    time.Duration
	NoFollow bool
}

// screen implements the Screen interface
type screen struct {
	client  relay.Client
	log     logger.Logger
	render  *render.Log
	format  string
	project string
	out     io.Writer
	width   func() int
}

// NewScreen creates a new logs screen
func NewScreen(client relay.Client, log logger.Logger, r *render.Log, identity instance.Identity, cfg *config.Config) Screen {
	return &screen{
		client:  client,
		log:     log.WithComponent("LOGS"),
		render:  r,
		format:  cfg.Logging.Format,
		project: identity.Fingerprint,
		out:     os.Stdout,
		width:   terminalWidth,
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
func (s *screen) Run(ctx context.Context, options StreamOptions) int {
	socketPath, err := relay.FindSocket(config.SocketDir, options.Profile)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to find socket")
		return 1
	}

	return s.streamLogs(ctx, socketPath, options)
}

// streamLogs connects to a running fuku instance and streams logs
func (s *screen) streamLogs(ctx context.Context, socketPath string, options StreamOptions) int {
	if err := s.client.Connect(socketPath); err != nil {
		s.log.Error().Err(err).Msg("Failed to connect to socket")
		return 1
	}

	defer s.client.Close()

	subscription := relay.SubscribeOptions{
		Services: options.Services,
		Tail:     options.Tail,
		Since:    options.Since,
		NoFollow: options.NoFollow,
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
		project:    s.project,
		following:  !options.NoFollow,
		out:        s.out,
		width:      s.width,
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
	project    string
	following  bool
	out        io.Writer
	width      func() int
}

// HandleStatus renders the connection banner
func (h *screenHandler) HandleStatus(status relay.StatusMessage) {
	h.render.RenderBanner(h.out, render.BannerOptions{
		Width:      h.width(),
		Status:     status,
		Subscribed: h.subscribed,
		Project:    h.project,
		Following:  h.following,
	})
}

// HandleLog writes a formatted log line
func (h *screenHandler) HandleLog(msg relay.LogMessage) {
	line := h.render.FormatMessage(h.format, msg.Service, msg.Message)
	//nolint:errcheck // best-effort write to output
	io.WriteString(h.out, line)
}
