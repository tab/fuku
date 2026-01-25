package main

import "examples/bookstore/shared"

var config = shared.Config{
	Name: "FRONTEND-API",
	Messages: []string{
		"INFO: [FRONTEND-API] Rendered homepage in 45ms",
		"DEBUG: [FRONTEND-API] Template cache hit",
		"INFO: [FRONTEND-API] Search results returned: 42 books",
		"WARN: [FRONTEND-API] Slow render detected (320ms)",
	},
}

func main() {
	shared.Run(config)
}
