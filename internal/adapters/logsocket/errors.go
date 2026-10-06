package logsocket

import "errors"

// errStatusMissing reports a server whose first frame is not a status
var errStatusMissing = errors.New("the running instance sent no status")

// errStopNotAcknowledged reports a server that answered a stop request with another frame
var errStopNotAcknowledged = errors.New("the running instance did not acknowledge the stop")
