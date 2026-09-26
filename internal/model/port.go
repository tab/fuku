package model

// Port is the address a readiness probe listens on and whether something already answers there
type Port struct {
	Address string
	InUse   bool
}
