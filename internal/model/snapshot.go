package model

import "time"

// Snapshot is the runtime read model every frontend shares (a tier and the map point at the same service)
type Snapshot struct {
	Phase     Phase
	Profile   string
	Resolved  bool
	StartedAt time.Time
	API       API
	Tiers     []*Tier
	Services  map[string]*Service
}

// API is the REST API listening state and its bound address
type API struct {
	Listening bool
	Address   string
}

// Counts tallies the services by status
func (s *Snapshot) Counts() Counts {
	counts := Counts{Total: len(s.Services)}

	for _, svc := range s.Services {
		switch svc.Status {
		case StatusPending:
			counts.Pending++
		case StatusStarting:
			counts.Starting++
		case StatusRunning:
			counts.Running++
		case StatusStopping:
			counts.Stopping++
		case StatusRestarting:
			counts.Restarting++
		case StatusStopped:
			counts.Stopped++
		case StatusFailed:
			counts.Failed++
		}
	}

	return counts
}

// Counts contains service counts grouped by status
type Counts struct {
	Total      int
	Pending    int
	Starting   int
	Running    int
	Stopping   int
	Restarting int
	Stopped    int
	Failed     int
}
