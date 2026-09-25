package contracts

import "errors"

// Outcomes that cross a layer; adapter-local sentinels live beside their adapter
var (
	ErrFailedToReadConfig       = errors.New("failed to read config file")
	ErrInvalidConfig            = errors.New("invalid configuration")
	ErrNoServicesDefined        = errors.New("no services defined")
	ErrProfileNotFound          = errors.New("profile not found")
	ErrServiceNotFound          = errors.New("service not found")
	ErrServiceDirectoryNotExist = errors.New("service directory does not exist")

	ErrInstanceAlreadyRunning  = errors.New("fuku is already running for this project")
	ErrNoInstanceRunning       = errors.New("no fuku instance is running")
	ErrProfileMismatch         = errors.New("fuku is running another profile")
	ErrBoundedReadNotSupported = errors.New("the running fuku instance does not support --tail and --no-follow")

	ErrPortAlreadyInUse      = errors.New("port already in use")
	ErrReadinessTimeout      = errors.New("readiness check timed out")
	ErrProcessExited         = errors.New("process exited before readiness")
	ErrUnexpectedExit        = errors.New("process exited")
	ErrMaxRetriesExceeded    = errors.New("max retry attempts exceeded")
	ErrFailedToAcquireWorker = errors.New("failed to acquire worker")
	ErrFailedToStartCommand  = errors.New("failed to start command")

	ErrNotAccepting     = errors.New("instance is not accepting actions")
	ErrServiceBusy      = errors.New("service is busy")
	ErrActionNotAllowed = errors.New("action not allowed")

	ErrBusOverloaded = errors.New("bus overloaded")
	ErrBusClosed     = errors.New("bus closed")
)
