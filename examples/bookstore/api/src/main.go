package main

import (
	"examples/bookstore/pkg/common"
)

var config = common.Config{
	Name:     "API",
	HTTPPort: 8080,
	Messages: []string{
		"INFO: [API] Processing request GET /api/v1/books",
		"DEBUG: [API] Cache hit for user data",
		"WARN: [API] Slow query detected (245ms)",
		"INFO: [API] Health check OK",
	},
}

func main() {
	common.Run(config)
}
