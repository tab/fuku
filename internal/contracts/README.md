# contracts

The vocabulary the rings share: every bus message, the bus protocol, the live process handle and the errors that cross a layer.
It imports `model` and the standard library. It imports no Fx and no package of an outer ring.

`platform/bus` moves the messages. `contracts` says what they are and which ones must arrive.

## Messages

One file holds one message or protocol type. A message is a `MessageType` string plus a payload struct.

- an event is `Event<Object><State>` with a past participle or adjective (`EventServiceStopped`, `EventTierReady`).
  Its payload is the struct of the same name minus the kind (`ServiceStopped`)
- a command is `Command<Verb><Object>` (`CommandStartService`, `CommandStopAll`). Its payload is the `model.Service` it targets.
  `CommandStopAll` targets every service and carries no payload
- the wire string is the identifier minus the kind, in snake_case (`service_stopped`, `cmd_stop_all`).
  The event log writes it and log clients read it, so a wire string is frozen once it ships.
  A renamed identifier keeps its old value with a `frozen wire value` comment
- a payload carries what a subscriber needs to act. It never calls back into the publisher.
  The service events embed `ServiceEvent`, which names the service and its tier
- `MessageType.Critical()` is a fixed table. A critical message must reach every required subscription.
  A non-critical one is dropped where a queue is full. `Test_MessageType_Critical` pins every row and its wire string.
  A type missing from the table reads as non-critical, so class it when you add it

## The bus protocol

A package takes `Publisher` and `Subscriber`, never the bus. `Subscribe` returns a `Subscription` that delivers messages in publication order.
`SubscribeOptions` names it, marks it `Required` when a critical message must not be lost, and filters it with `Types`.

`Run` drives a subscription on its own goroutine and calls the handler for every message.
It returns a `Loop`. `Drain` waits until the queue is empty and no handler is in flight. `Done` closes once the goroutine has exited.

The loop is the only receiver of the channel. That is what makes an empty channel mean idle.

## The process handle

`Process` is the live child of one service. `adapters/process` creates and tracks it, `adapters/readiness` reads its streams, `app/services` drives it.
It lives here so the core can hold a child without importing its maker.

## Errors

The sentinels here are the outcomes that cross a layer: a config that did not load, a readiness timeout, a rejected action, an overloaded bus.
A frontend maps them: REST to a status code, the TUI to a log line, the CLI to an exit code.
`ActionNotAllowedError` carries the rejected `Action`; `errors.Is` sees `ErrActionNotAllowed` and `errors.As` yields the action.

A sentinel only one adapter raises and handles lives in an `errors.go` beside that adapter.

## Changing it

- a new message gets its own file, a row in the critical table and a row in the bus table of `ARCHITECTURE.md`
- a wire string never changes. Rename the identifier, keep the value, add the comment
- before a new type, check whether an existing payload can carry the data. Extend that struct
- a handler runs on the loop goroutine. A slow handler fills its own queue and, when the subscription is required, fails the next critical publish
- nothing here decides anything. A rule about a message belongs to the package that publishes or consumes it
