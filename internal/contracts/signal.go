package contracts

// EventSignalReceived carries a SignalReceived
const EventSignalReceived MessageType = "signal" // frozen wire value, log consumers read it

// SignalReceived contains information about a received OS signal
type SignalReceived struct {
	Name string
}
