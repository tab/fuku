package contracts

// Resource sampling event types
const (
	EventResourceSampled         MessageType = "resource_sample" // frozen wire value, log consumers read it
	EventServiceResourcesSampled MessageType = "service_resources_sampled"
)

// ResourceSampled contains fuku process CPU and memory readings
type ResourceSampled struct {
	CPU    float64
	Memory uint64
}

// ServiceResourcesSampled carries one reading per running service process, taken in the same tick
type ServiceResourcesSampled struct {
	Services []ServiceResourceSample
}

// ServiceResourceSample is the usage of one service process, valid only while the service still runs that PID
type ServiceResourceSample struct {
	ID     string
	PID    int
	CPU    float64
	Memory uint64
}
