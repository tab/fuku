package detach

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"fuku/internal/contracts"
	"fuku/internal/model"
)

// Relay is the log socket server a detached start waits on before it reports success
type Relay interface {
	Bound(ctx context.Context) error
}

// API is the REST server a detached start reports the address of, when the project serves one
type API interface {
	Address(ctx context.Context) string
}

// Reporter ends the run with a failure
type Reporter interface {
	Fail(err error)
}

// Output is where the child writes its records, released once the start succeeded
type Output interface {
	io.Writer
	Release() error
}

// Logger is the logging surface of the package
type Logger interface {
	Warn(msg string, args ...any)
}

// progressTypes lists the events that make up a detached start
var progressTypes = []contracts.MessageType{
	contracts.EventProfileResolved,
	contracts.EventServiceStarting,
	contracts.EventServiceReady,
	contracts.EventServiceFailed,
	contracts.EventServiceStopped,
	contracts.EventPhaseChanged,
}

// Progress reports the startup of the detached child to its parent and fails the run on the first lost service
type Progress struct {
	subscriber contracts.Subscriber
	relay      Relay
	api        API
	reporter   Reporter
	output     Output
	loop       *contracts.Loop
	settled    bool
	log        Logger
}

// NewProgress creates the startup reporter of the detached child (api is nil when the project serves no API)
func NewProgress(subscriber contracts.Subscriber, relay Relay, api API, reporter Reporter, output Output, log Logger) *Progress {
	return &Progress{
		subscriber: subscriber,
		relay:      relay,
		api:        api,
		reporter:   reporter,
		output:     output,
		log:        log,
	}
}

// Subscribe registers the required subscription and reports the startup on its own goroutine
func (p *Progress) Subscribe(ctx context.Context) error {
	ignoreBrokenPipe()

	sub, err := p.subscriber.Subscribe(ctx, contracts.SubscribeOptions{Name: "detach", Required: true, Types: progressTypes})
	if err != nil {
		return fmt.Errorf("failed to subscribe the detached start: %w", err)
	}

	p.loop = contracts.Run(ctx, sub, func(msg contracts.Message) {
		p.handle(ctx, msg)
	})

	return nil
}

// Drain reports the queued events and returns once none is in flight (or once ctx expires)
func (p *Progress) Drain(ctx context.Context) error {
	return p.loop.Drain(ctx)
}

// ignoreBrokenPipe turns a record write to a parent that went away into EPIPE instead of a SIGPIPE that kills the child
func ignoreBrokenPipe() {
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
}

// handle turns one event into a record until the start succeeded or failed
func (p *Progress) handle(ctx context.Context, msg contracts.Message) {
	if p.settled {
		return
	}

	switch data := msg.Data.(type) {
	case contracts.ProfileResolved:
		p.write(Record{Kind: KindProfile, Services: names(data.Tiers)})
	case contracts.ServiceStarting:
		p.write(Record{Kind: KindStarting, Service: data.Service.Name})
	case contracts.ServiceReady:
		p.write(Record{Kind: KindReady, Service: data.Service.Name, Duration: data.Duration})
	case contracts.ServiceFailed:
		p.fail(data.Service.Name, data.Error)
	case contracts.ServiceStopped:
		if data.Unexpected {
			p.fail(data.Service.Name, contracts.ErrUnexpectedExit)
		}
	case contracts.PhaseChanged:
		if data.Phase == model.PhaseRunning {
			p.succeed(ctx, data)
		}
	}
}

// succeed reports the running instance once the log socket and the API are bound, then releases the pipe to the parent
func (p *Progress) succeed(ctx context.Context, phase contracts.PhaseChanged) {
	p.settled = true

	if err := p.relay.Bound(ctx); err != nil {
		p.reporter.Fail(fmt.Errorf("failed to start the logs server: %w", err))

		return
	}

	p.write(Record{Kind: KindRunning, PID: os.Getpid(), Count: phase.ServiceCount, Duration: phase.Duration, Address: p.address(ctx)})

	if err := p.output.Release(); err != nil {
		p.reporter.Fail(fmt.Errorf("failed to release the detached start output: %w", err))
	}
}

// address returns the bound API address, empty when the project serves no API
func (p *Progress) address(ctx context.Context) string {
	if p.api == nil {
		return ""
	}

	return p.api.Address(ctx)
}

// fail reports the lost service and ends the run
func (p *Progress) fail(service string, err error) {
	p.settled = true

	p.write(Record{Kind: KindFailed, Service: service, Error: err.Error()})
	p.reporter.Fail(fmt.Errorf("service '%s' failed to start: %w", service, err))
}

// write sends one record to the parent
func (p *Progress) write(record Record) {
	if _, err := p.output.Write(encode(record)); err != nil {
		p.log.Warn("Failed to report the detached start", "error", err)
	}
}

// names lists the service names of the profile in startup order
func names(tiers model.Tiers) []string {
	services := tiers.Services()
	result := make([]string, 0, len(services))

	for _, svc := range services {
		result = append(result, svc.Name)
	}

	return result
}
