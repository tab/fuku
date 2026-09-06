package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// Probe limits for the liveness request
const (
	ProbeTimeout      = 250 * time.Millisecond
	ProbeResponseSize = 4096
)

// livePath is the unauthenticated liveness endpoint
const livePath = "/api/v1/live"

// refusalFormat is the message shown when another instance owns the project
const refusalFormat = "Error: %v (API at %s)\nRun 'fuku logs' to follow it or stop that instance before starting another.\n"

// liveResponse is the part of the liveness payload the probe reads
type liveResponse struct {
	Product     string `json:"product"`
	Fingerprint string `json:"fingerprint"`
}

// Guard refuses a run when the configured API port range already serves this project
type Guard interface {
	Check(ctx context.Context) error
}

type guard struct {
	cfg        *config.Config
	identity   Identity
	stderr     io.Writer
	httpClient *http.Client
}

// NewGuard creates the single-instance guard for the current project
func NewGuard(cfg *config.Config, identity Identity, stderr io.Writer) Guard {
	return &guard{
		cfg:      cfg,
		identity: identity,
		stderr:   stderr,
		httpClient: &http.Client{
			Timeout: ProbeTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Check returns ErrInstanceAlreadyRunning when another instance serves this project
func (g *guard) Check(ctx context.Context) error {
	address := g.find(ctx, g.cfg.ServerListen())
	if address == "" {
		return nil
	}

	fmt.Fprintf(g.stderr, refusalFormat, errors.ErrInstanceAlreadyRunning, address)

	return errors.ErrInstanceAlreadyRunning
}

// find returns the first address in the configured port range that serves this project
func (g *guard) find(ctx context.Context, listen string) string {
	host, portStr, err := net.SplitHostPort(listen)
	if err != nil {
		return ""
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ""
	}

	for offset := range config.APIPortRetries {
		address := net.JoinHostPort(host, strconv.Itoa(port+offset))

		if g.matches(ctx, address) {
			return address
		}
	}

	return ""
}

// matches reports whether the address serves this project
func (g *guard) matches(ctx context.Context, address string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+livePath, nil)
	if err != nil {
		return false
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ProbeResponseSize+1))
	if err != nil || len(body) > ProbeResponseSize {
		return false
	}

	var live liveResponse
	if err := json.Unmarshal(body, &live); err != nil {
		return false
	}

	return live.Product == config.AppName && live.Fingerprint == g.identity.Fingerprint
}
