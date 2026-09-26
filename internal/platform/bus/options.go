package bus

// QueueDepthDefault is the number of messages a subscription buffers
const QueueDepthDefault = 1024

// Options configures the bus transport
type Options struct {
	QueueDepth int
}
