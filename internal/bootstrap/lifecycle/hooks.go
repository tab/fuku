package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.uber.org/fx"

	"fuku/internal/contracts"
)

// Guard refuses the run when another instance already serves the project
type Guard interface {
	Check(ctx context.Context) error
}

// Consumer registers its named, filtered subscription before any producer starts and drains it at shutdown
type Consumer interface {
	Subscribe(ctx context.Context) error
	Drain(ctx context.Context) error
}

// Producer starts once every consumer is subscribed and stops before the consumers drain
type Producer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Command is the one command a composition runs; its return code is the exit code and its error the reported cause
type Command interface {
	Run(ctx context.Context) (int, error)
}

// Closer closes the bus once every consumer drained
type Closer interface {
	Close()
}

// Telemetry spans the run: it starts before the first consumer, stops after the last drain and reports a panic
type Telemetry interface {
	Start()
	Stop()
	Recover(r any)
}

// Logger is the logging surface the coordinator writes through
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Participants are the values one composition hands to the coordinator (the guard is nil where a composition has none)
type Participants struct {
	Guard     Guard
	Consumers []Consumer
	Producers []Producer
	Command   Command
}

// Coordinator drives the participants through startup and shutdown on its own context, apart from the Fx hooks
type Coordinator struct {
	//nolint:containedctx // the coordinator owns the run-wide context
	ctx          context.Context
	cancel       context.CancelFunc
	arbiter      *Arbiter
	publisher    contracts.Publisher
	closer       Closer
	telemetry    Telemetry
	participants Participants
	started      []Producer
	done         chan struct{}
	log          Logger
}

// NewCoordinator creates the coordinator of one composition
func NewCoordinator(arbiter *Arbiter, publisher contracts.Publisher, closer Closer, telemetry Telemetry, participants Participants, log Logger) *Coordinator {
	return &Coordinator{
		arbiter:      arbiter,
		publisher:    publisher,
		closer:       closer,
		telemetry:    telemetry,
		participants: participants,
		log:          log,
	}
}

// Register appends the coordinator to the Fx lifecycle
func Register(lc fx.Lifecycle, coordinator *Coordinator) {
	lc.Append(fx.Hook{
		OnStart: coordinator.Start,
		OnStop:  coordinator.Stop,
	})
}

// Start runs the guard, telemetry, consumers, producers and command in that order, unwinding on a failure
func (c *Coordinator) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(context.Background())

	if err := c.check(ctx); err != nil {
		c.unwind(ctx)

		return err
	}

	c.telemetry.Start()

	for _, consumer := range c.participants.Consumers {
		if err := consumer.Subscribe(c.ctx); err != nil {
			c.unwind(ctx)

			return err
		}
	}

	for _, producer := range c.participants.Producers {
		if err := producer.Start(c.ctx); err != nil {
			c.unwind(ctx)

			return err
		}

		c.started = append(c.started, producer)
	}

	c.done = make(chan struct{})

	go c.run(c.ctx)

	return nil
}

// check runs the guard when the composition has one
func (c *Coordinator) check(ctx context.Context) error {
	if c.participants.Guard == nil {
		return nil
	}

	return c.participants.Guard.Check(ctx)
}

// Stop announces the signal, stops producers, drains consumers, joins the command, stops telemetry and closes the bus
func (c *Coordinator) Stop(ctx context.Context) error {
	c.publishSignal()

	var errs []error

	for _, producer := range slices.Backward(c.started) {
		if err := producer.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// ponytail: two drain passes miss a publish back to an earlier consumer; loop until idle if one ever does
	for range 2 {
		for _, consumer := range c.participants.Consumers {
			if err := consumer.Drain(ctx); err != nil {
				errs = append(errs, err)

				break
			}
		}
	}

	c.cancel()

	select {
	case <-c.done:
	case <-ctx.Done():
		errs = append(errs, ctx.Err())
	}

	c.telemetry.Stop()
	c.closer.Close()

	return errors.Join(errs...)
}

// run executes the command and records the outcome
func (c *Coordinator) run(ctx context.Context) {
	defer close(c.done)

	defer func() {
		if r := recover(); r != nil {
			c.telemetry.Recover(r)
			panic(r)
		}
	}()

	c.arbiter.decide(c.participants.Command.Run(ctx))
}

// unwind stops the producers a failed start already started, newest first, cancels, stops telemetry, closes the bus
func (c *Coordinator) unwind(ctx context.Context) {
	for _, producer := range slices.Backward(c.started) {
		if err := producer.Stop(ctx); err != nil {
			c.log.Error("Failed to stop a producer while unwinding the start", "error", err)
		}
	}

	c.started = nil
	c.cancel()
	c.telemetry.Stop()
	c.closer.Close()
}

// publishSignal announces the OS signal that initiated the shutdown, when one did
func (c *Coordinator) publishSignal() {
	sig := c.arbiter.signal()
	if sig == nil {
		return
	}

	c.log.Info(fmt.Sprintf("Received signal %s, shutting down services...", sig))

	//nolint:errcheck // a non-critical publish never fails
	c.publisher.Publish(contracts.Message{
		Type: contracts.EventSignalReceived,
		Data: contracts.SignalReceived{Name: sig.String()},
	})
}
