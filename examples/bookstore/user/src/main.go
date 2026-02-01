package main

import (
	"examples/bookstore/pkg/common"
)

var config = common.Config{
	Name:    "USER",
	TCPPort: 9090,
	Messages: []string{
		"INFO: [USER] Profile updated for user_456",
		"DEBUG: [USER] Preferences cached",
		"INFO: [USER] Password changed successfully",
		"WARN: [USER] Invalid email format detected",
	},
}

func main() {
	common.Run(config)
}
