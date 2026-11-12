#!/bin/sh
# api watches its code and panics on an off-by-one four seconds after it is ready
set -eu
cat > fuku.override.yaml <<'YAML'
services:
  api:
    watch:
      include: ["**/*.go"]
      shared: ["examples/bookstore/pkg/common"]
      debounce: 500ms
YAML
cat > examples/bookstore/api/src/main.go <<'GO'
package main

import (
	"fmt"
	"time"

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
	go refreshBestsellers()
	common.Run(config)
}

func refreshBestsellers() {
	time.Sleep(4 * time.Second)
	books := []string{"Dune", "Neuromancer", "Snow Crash"}
	latest := books[len(books)]
	fmt.Printf("INFO: [API] Bestseller cache refreshed, latest %s\n", latest)
}
GO
