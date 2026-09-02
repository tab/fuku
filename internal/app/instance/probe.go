package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"fuku/internal/app/errors"
	"fuku/internal/config"
)

// livePath is the unauthenticated endpoint that identifies a fuku instance
const livePath = "/api/v1/live"

// live is the subset of the liveness payload that identifies an instance
type live struct {
	Product  string `json:"product"`
	Instance string `json:"instance"`
	Project  string `json:"project"`
}

// Instance describes a fuku API answering on a loopback address
// (Project is empty when the instance predates project identity)
type Instance struct {
	Address string
	ID      string
	Project string
}

// Running reports the address of a live fuku instance already serving the given project
// (an unreachable or unidentifiable API is reported as no instance, because neither can be attached to)
func Running(ctx context.Context, listen, fingerprint string) (string, bool) {
	found := false
	address := ""

	scan(ctx, listen, func(candidate Instance) bool {
		if candidate.Project != fingerprint {
			return true
		}

		address, found = candidate.Address, true

		return false
	})

	return address, found
}

// Scan reports every fuku instance answering in the port range for the configured listen address
func Scan(ctx context.Context, listen string) []Instance {
	var instances []Instance

	scan(ctx, listen, func(candidate Instance) bool {
		instances = append(instances, candidate)

		return true
	})

	return instances
}

// scan walks the port range fuku may have bound, stopping when visit reports it has seen enough
func scan(ctx context.Context, listen string, visit func(Instance) bool) {
	host, port, err := SplitListen(listen)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: config.APIProbeTimeout}

	for i := range config.APIPortRetries {
		address := net.JoinHostPort(host, strconv.Itoa(port+i))

		found, ok := identify(ctx, client, address)
		if !ok {
			continue
		}

		if !visit(found) {
			return
		}
	}
}

// identify asks one address whether it is a fuku API and which project it serves
func identify(ctx context.Context, client *http.Client, address string) (Instance, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+livePath, nil)
	if err != nil {
		return Instance{}, false
	}

	resp, err := client.Do(req)
	if err != nil {
		return Instance{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Instance{}, false
	}

	var payload live
	if err := json.NewDecoder(io.LimitReader(resp.Body, config.APIProbeBodyLimit)).Decode(&payload); err != nil {
		return Instance{}, false
	}

	if payload.Product != config.AppName {
		return Instance{}, false
	}

	return Instance{Address: address, ID: payload.Instance, Project: payload.Project}, true
}

// SplitListen separates a configured listen address into its host and port
func SplitListen(listen string) (string, int, error) {
	host, portText, err := net.SplitHostPort(listen)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", errors.ErrAPIInvalidListen, err)
	}

	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", errors.ErrAPIInvalidListen, err)
	}

	return host, port, nil
}
