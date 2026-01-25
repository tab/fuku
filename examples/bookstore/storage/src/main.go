package main

import "examples/bookstore/shared"

var config = shared.Config{
	Name: "STORAGE",
	Messages: []string{
		"INFO: [STORAGE] File uploaded: book_cover_123.jpg",
		"DEBUG: [STORAGE] Cache hit for asset bucket",
		"INFO: [STORAGE] Cleanup completed, freed 150MB",
		"DEBUG: [STORAGE] Replication sync complete",
	},
}

func main() {
	shared.Run(config)
}
