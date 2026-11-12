# contracts

The vocabulary the rings share: every bus message, the bus protocol, the live process handle, the log socket's status handshake and the errors that cross a layer.
It imports `model` and the standard library only, and no Fx. `platform/bus` moves the messages.
`contracts` says what they are and which ones must arrive.

## Messages

One file holds the messages of one object or protocol area (`service.go`, `tier.go`, `watch.go`).
A message is a `MessageType` string plus a payload struct.
The names follow the constant naming rules in [`CLAUDE.md`](../../CLAUDE.md#constant-naming).

- an event's payload is the struct of the same name minus the kind: `EventServiceStopped` carries `ServiceStopped`
- a command's payload is the `model.Service` it targets. `CommandStopAll` targets every service and carries no payload
- the event log writes the wire string and log clients read it, so a wire string is frozen once it ships
- a payload carries what a subscriber needs to act. It never calls back into the publisher.
  The service events embed `ServiceEvent`, which names the service and its tier
- `MessageType.Critical()` is a fixed table. A critical message must reach every required subscription.
  A non-critical one is dropped where a queue is full. `Test_MessageType_Critical` pins every row and its wire string.
  A type missing from the table reads as non-critical, so class it when you add it

## The bus protocol

A package takes `Publisher` and `Subscriber`, never the bus.
`Subscribe` returns a `Subscription` that delivers messages in publication order.
`SubscribeOptions` names it, marks it `Required` when a critical message must not be lost, and filters it with `Types`.

`Run` drives a subscription on its own goroutine and calls the handler for every message. It returns a `Loop`.
`Drain` waits until the queue is empty and no handler is in flight.
The loop is the only receiver of the channel, so an empty channel means idle.

## The process handle

`Process` is the live child of one service.
`adapters/process` creates and tracks it, `adapters/readiness` reads its streams, `app/services` drives it.
It lives here so the core can hold a child without importing its maker.

`Stdout` and `Stderr` are copies of the child's streams for a `log` readiness probe.
The child waits on them until the probe reads or closes them. Without a `log` probe they are closed from the start.

## Errors

The sentinels here are the outcomes that cross a layer: a config that did not load, a readiness timeout, a rejected action, an overloaded bus, an answer of the REST API.
A frontend maps them: REST to a status code, the TUI to a log line, the CLI to an exit code.
A sentinel that only one adapter raises and handles lives in an `errors.go` beside that adapter.

## Changing it

- a new message joins its object's file, or starts one, plus a row in the critical table and a row in the bus table of `ARCHITECTURE.md`
- a wire string never changes. Rename the identifier, keep the value, add a `frozen wire value` comment
- before a new type, check whether an existing payload can carry the data. Extend that struct
- a handler runs on the loop goroutine. A slow handler fills its own queue and, when the subscription is required, fails the next critical publish
- nothing here decides anything. A rule about a message belongs to the package that publishes or consumes it
