package bus

import "errors"

// ErrUnnamedSubscription rejects a subscription without a name
var ErrUnnamedSubscription = errors.New("subscription name is required")
