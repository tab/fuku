package model

// Instance identifies one running fuku instance and the project directory it serves
type Instance struct {
	ID          string
	Project     string
	Fingerprint string
}
