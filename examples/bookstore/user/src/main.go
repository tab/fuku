package main

import "examples/bookstore/shared"

var config = shared.Config{
	Name: "USER",
	Messages: []string{
		"INFO: [USER] Profile updated for user_456",
		"DEBUG: [USER] Preferences cached",
		"INFO: [USER] Password changed successfully",
		"WARN: [USER] Invalid email format detected",
	},
}

func main() {
	shared.Run(config)
}
