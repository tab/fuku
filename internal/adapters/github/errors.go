package github

import "errors"

// Sentinels local to the github adapter
var (
	ErrEmptyReleaseTag         = errors.New("empty tag in release response")
	ErrUnexpectedReleaseStatus = errors.New("unexpected release response status")
)
