package rest

import "time"

// PortRetries is the number of consecutive ports tried from the configured one
const PortRetries = 10

// readHeaderTimeout bounds the wait for a request header
const readHeaderTimeout = 5 * time.Second

// Options is the bind address and the bearer token of the REST API (an empty Listen disables the server)
type Options struct {
	Listen string
	Token  string
}
