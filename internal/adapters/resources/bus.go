package resources

import "fuku/internal/contracts"

// publishProcess announces one sample of the fuku process usage
func (s *Sampler) publishProcess(stats Stats) {
	//nolint:errcheck // a non-critical publish never fails
	s.publisher.Publish(contracts.Message{
		Type: contracts.EventResourceSampled,
		Data: contracts.ResourceSampled{
			CPU: stats.CPU,
			MEM: stats.MEM,
		},
	})
}

// publishServices announces one batched sample of the service processes
func (s *Sampler) publishServices(samples []contracts.ServiceResourceSample) {
	//nolint:errcheck // a non-critical publish never fails
	s.publisher.Publish(contracts.Message{
		Type: contracts.EventServiceResourcesSampled,
		Data: contracts.ServiceResourcesSampled{Services: samples},
	})
}
