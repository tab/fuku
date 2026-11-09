package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"fuku/internal/contracts"
	"fuku/internal/model"
	"fuku/internal/platform/buildinfo"
)

// requestTimeout bounds one read request to the API
const requestTimeout = 3 * time.Second

// actionMargin is how long an action request may outlive the wait it asks for
const actionMargin = 5 * time.Second

// API paths the client calls
const (
	pathLive     = "/api/v1/live"
	pathStatus   = "/api/v1/status"
	pathServices = "/api/v1/services"
)

// Loopback host names a client address may use
const (
	hostLocalhost    = "localhost"
	hostIP6Localhost = "ip6-localhost"
)

// HTTPDoer sends one HTTP request; *http.Client satisfies it
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client calls the REST API of a running instance at one loopback address, with the token when it is set
type Client struct {
	http  HTTPDoer
	base  string
	token string
}

// NewHTTPClient creates the HTTP client of the API calls, which never follows a redirect with the token
func NewHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewClient creates the client of a loopback address, mapping ip6-localhost to ::1 as the config accepts it
func NewClient(doer HTTPDoer, address, token string) (*Client, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrClientNotLoopback, address)
	}

	if host == hostIP6Localhost {
		host = net.IPv6loopback.String()
	}

	ip := net.ParseIP(host)
	loopback := host == hostLocalhost || (ip != nil && ip.IsLoopback())

	if !loopback {
		return nil, fmt.Errorf("%w: %s", ErrClientNotLoopback, address)
	}

	return &Client{http: doer, base: "http://" + net.JoinHostPort(host, port), token: token}, nil
}

// Live reads the unauthenticated liveness answer and fails when the address serves another product
func (c *Client) Live(ctx context.Context) (model.Instance, error) {
	var live LiveSerializer
	if err := c.get(ctx, pathLive, &live); err != nil {
		return model.Instance{}, err
	}

	if live.Product != buildinfo.AppName {
		return model.Instance{}, fmt.Errorf("%w: %s", ErrClientNotFuku, live.Product)
	}

	return model.Instance{ID: live.Instance, Fingerprint: live.Fingerprint}, nil
}

// Status reads the running profile and its phase
func (c *Client) Status(ctx context.Context) (model.Snapshot, error) {
	var status StatusSerializer
	if err := c.get(ctx, pathStatus, &status); err != nil {
		return model.Snapshot{}, err
	}

	return model.Snapshot{Profile: status.Profile, Phase: model.Phase(status.Phase)}, nil
}

// Services reads every service of the running profile in the order the API lists them
func (c *Client) Services(ctx context.Context) ([]model.Service, error) {
	var list ServiceListSerializer
	if err := c.get(ctx, pathServices, &list); err != nil {
		return nil, err
	}

	services := make([]model.Service, 0, len(list.Services))
	for _, service := range list.Services {
		services = append(services, fromServiceSerializer(service))
	}

	return services, nil
}

// Service reads the current state of one service
func (c *Client) Service(ctx context.Context, id string) (model.Service, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.send(ctx, http.MethodGet, servicePath(id), nil)
	if err != nil {
		return model.Service{}, err
	}

	defer resp.Body.Close()

	return decodeService(resp)
}

// Act applies the action and waits for the service to settle; settled is false when the wait ends first
func (c *Client) Act(ctx context.Context, id string, action contracts.Action, wait time.Duration) (model.Service, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, wait+actionMargin)
	defer cancel()

	seconds := int(wait / time.Second)

	body, _ := json.Marshal(ActionRequest{Wait: &seconds})

	resp, err := c.send(ctx, http.MethodPost, servicePath(id)+"/"+string(action), bytes.NewReader(body))
	if err != nil {
		return model.Service{}, false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return model.Service{}, false, nil
	}

	service, err := decodeService(resp)
	if err != nil {
		return model.Service{}, false, err
	}

	return service, true, nil
}

// get reads one JSON answer of a read route
func (c *Client) get(ctx context.Context, path string, answer any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.send(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return contracts.ErrAPIUnauthorized
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: %s answered %s", ErrClientFailed, path, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(answer); err != nil {
		return fmt.Errorf("failed to decode the API answer of %s: %w", path, err)
	}

	return nil
}

// send makes one request, with the token when it is set and no URL in an error
func (c *Client) send(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build the API request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)

	if failure, ok := errors.AsType[*url.Error](err); ok {
		err = failure.Err
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %w", contracts.ErrAPIUnreachable, err)
	}

	return resp, nil
}

// servicePath is the API path of one service
func servicePath(id string) string {
	return pathServices + "/" + url.PathEscape(id)
}

// decodeService reads the service a service route answers, or maps its rejection onto a contracts error
func decodeService(resp *http.Response) (model.Service, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		var service ServiceSerializer
		if err := json.NewDecoder(resp.Body).Decode(&service); err != nil {
			return model.Service{}, fmt.Errorf("failed to decode the API answer: %w", err)
		}

		return fromServiceSerializer(service), nil
	case http.StatusUnauthorized:
		return model.Service{}, contracts.ErrAPIUnauthorized
	case http.StatusNotFound, http.StatusConflict:
		rejection := ErrorSerializer{Error: resp.Status}
		//nolint:errcheck // a plain-text rejection keeps its status as the text
		json.NewDecoder(resp.Body).Decode(&rejection)

		return model.Service{}, fmt.Errorf("%w: %s", contracts.ErrAPIRejected, rejection.Error)
	case http.StatusInternalServerError:
		return model.Service{}, contracts.ErrAPIOverloaded
	default:
		return model.Service{}, fmt.Errorf("%w: answered %s", ErrClientFailed, resp.Status)
	}
}

// fromServiceSerializer reads the service state an answer carries, turning its uptime back into a start time
func fromServiceSerializer(s ServiceSerializer) model.Service {
	service := model.Service{
		ID:       s.ID,
		Name:     s.Name,
		Tier:     s.Tier,
		Status:   s.Status,
		Watching: s.Watching,
		Error:    s.Error,
		Process:  model.Process{PID: s.PID, CPU: s.CPU, Memory: s.Memory},
	}

	if s.Uptime > 0 {
		service.Process.StartedAt = time.Now().Add(-time.Duration(s.Uptime) * time.Second)
	}

	return service
}
