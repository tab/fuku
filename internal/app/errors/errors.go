package errors

import (
	"errors"
)

var (
	ErrFailedToReadConfig        = errors.New("failed to read config file")
	ErrFailedToParseConfig       = errors.New("failed to parse config file")
	ErrInvalidConfig             = errors.New("invalid configuration")
	ErrInvalidConcurrencyWorkers = errors.New("concurrency workers must be greater than 0")
	ErrInvalidRetryAttempts      = errors.New("retry attempts must be greater than 0")
	ErrInvalidRetryBackoff       = errors.New("retry backoff must not be negative")
	ErrInvalidLogsBuffer         = errors.New("logs buffer must be greater than 0")

	ErrProfileNotFound          = errors.New("profile not found")
	ErrUnsupportedProfileFormat = errors.New("unsupported profile format")

	ErrServiceNotFound          = errors.New("service not found")
	ErrServiceNotInRegistry     = errors.New("service not found in registry")
	ErrServiceDirectoryNotExist = errors.New("service directory does not exist")

	ErrInvalidReadinessType     = errors.New("invalid readiness type")
	ErrReadinessTypeRequired    = errors.New("readiness type is required")
	ErrReadinessURLRequired     = errors.New("readiness type 'http' requires url field")
	ErrReadinessAddressRequired = errors.New("readiness type 'tcp' requires address field")
	ErrReadinessPatternRequired = errors.New("readiness type 'log' requires pattern field")
	ErrReadinessTimeout         = errors.New("readiness check timed out")
	ErrProcessExited            = errors.New("process exited before readiness")
	ErrInvalidRegexPattern      = errors.New("invalid regex pattern")
	ErrPortAlreadyInUse         = errors.New("port already in use")

	ErrWatchIncludeRequired = errors.New("watch configuration requires include field")
	ErrInvalidLogsOutput    = errors.New("invalid service logs output value (must be 'stdout' or 'stderr')")

	ErrFailedToGetWorkingDir = errors.New("failed to get working directory")
	ErrFailedToCreatePipe    = errors.New("failed to create pipe")
	ErrFailedToStartCommand  = errors.New("failed to start command")
	ErrFailedToCreateRequest = errors.New("failed to create request")

	ErrStartupInterrupted       = errors.New("startup interrupted")
	ErrCommandChannelClosed     = errors.New("command channel closed")
	ErrFailedToAcquireWorker    = errors.New("failed to acquire worker")
	ErrMaxRetriesExceeded       = errors.New("max retry attempts exceeded")
	ErrFailedToTerminateProcess = errors.New("failed to terminate process")
	ErrUnexpectedExit           = errors.New("process exited")

	ErrSessionNotFound  = errors.New("no active session found")
	ErrSessionCorrupted = errors.New("session file corrupted")
	ErrSessionDirCreate = errors.New("failed to create session directory")
	ErrSessionFileWrite = errors.New("failed to write session file")
	ErrSessionFileRead  = errors.New("failed to read session file")

	ErrFailedToConnectSocket    = errors.New("failed to connect to socket")
	ErrFailedToListenSocket     = errors.New("failed to listen on socket")
	ErrFailedToReadSocket       = errors.New("failed to read from socket")
	ErrFailedToWriteSocket      = errors.New("failed to write to socket")
	ErrFailedToMarshalMessage   = errors.New("failed to marshal message")
	ErrFailedToCleanupSocket    = errors.New("failed to cleanup stale socket")
	ErrSocketAlreadyInUse       = errors.New("socket is already in use")
	ErrSocketSearchFailed       = errors.New("failed to search for sockets")
	ErrNoInstanceRunning        = errors.New("no fuku instance is running")
	ErrMultipleInstancesRunning = errors.New("multiple fuku instances running")
	ErrInstanceNotFound         = errors.New("no fuku instance running with profile")
)

var (
	As  = errors.As
	Is  = errors.Is
	New = errors.New
)
