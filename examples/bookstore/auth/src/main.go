package main

import "examples/bookstore/shared"

var config = shared.Config{
	Name: "AUTH",
	Messages: []string{
		"INFO: [AUTH] Token validated for user_123",
		"DEBUG: [AUTH] JWT signature verified",
		"INFO: [AUTH] New session created",
		"WARN: [AUTH] Rate limit warning for IP 192.168.1.1",
	},
}

func main() {
	shared.Run(config)
}
