package logsocket

import "errors"

// errStatusMissing reports a server whose first frame is not a status
var errStatusMissing = errors.New("the running instance sent no status")
