#!/bin/sh
# api exits unless BOOKS_DB_URL is set, and only .env.example holds it
set -eu
api=examples/bookstore/api
cat > $api/src/main.go <<'GO'
package main

import (
	"fmt"
	"os"

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
	if os.Getenv("BOOKS_DB_URL") == "" {
		fmt.Println("ERROR: [API] BOOKS_DB_URL is not set")
		os.Exit(1)
	}
	common.Run(config)
}
GO
printf '.PHONY: run\nrun:\n\t@set -a; [ -f .env ] && . ./.env; set +a; go build -o /tmp/fuku-api src/main.go && exec /tmp/fuku-api\n' > $api/Makefile
printf 'BOOKS_DB_URL=postgres://localhost:5432/books\n' > $api/.env.example
