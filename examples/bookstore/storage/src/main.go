package main

import (
	"examples/bookstore/pkg/common"
)

var config = common.Config{
	Name: "STORAGE",
	Messages: []string{
		"INFO: [STORAGE] File uploaded: book_cover_123.jpg",
		"DEBUG: [STORAGE] Cache hit for asset bucket",
		"INFO: [STORAGE] Cleanup completed, freed 150MB",
		"DEBUG: [STORAGE] Replication sync complete",
	},
}

func main() {
	common.Run(config)
}
