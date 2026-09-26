package envfiles

import "errors"

// ErrUnsafePath reports an env.files entry that is absolute or leaves the service directory
var ErrUnsafePath = errors.New("env file path leaves the service directory")
