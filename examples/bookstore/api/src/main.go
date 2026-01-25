package main

import "examples/bookstore/shared"

var config = shared.Config{
	Name: "API",
	Messages: []string{
		"INFO: [API] Processing request GET /api/v1/books",
		"DEBUG: [API] Cache hit for user data",
		"WARN: [API] Slow query detected (245ms)",
		"INFO: [API] Health check OK",
	},
}

func main() {
	shared.Run(config)
}
