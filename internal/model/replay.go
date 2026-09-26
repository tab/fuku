package model

// ReplayOptions bounds the buffered replay a client receives (shared by the CLI, the log screen and the wire messages)
type ReplayOptions struct {
	Tail     *int `json:"tail,omitempty"`
	NoFollow bool `json:"noFollow,omitempty"`
}

// Bounded reports whether the options ask for a bounded read
func (o ReplayOptions) Bounded() bool {
	return o.Tail != nil || o.NoFollow
}

// Equal reports whether both options describe the same replay (a nil tail only matches a nil tail)
func (o ReplayOptions) Equal(other ReplayOptions) bool {
	if o.NoFollow != other.NoFollow {
		return false
	}

	if o.Tail == nil || other.Tail == nil {
		return o.Tail == nil && other.Tail == nil
	}

	return *o.Tail == *other.Tail
}
