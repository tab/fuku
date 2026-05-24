package main

import (
	"examples/bookstore/pkg/common"
)

var config = common.Config{
	Name:    "WORKER",
	TCPPort: 9091,
	Messages: []string{
		"INFO: [WORKER] Task started for job ID 12345",
		"DEBUG: [WORKER] Job description for job ID 12345: process order #98765",
		"INFO: [WORKER] Task completed successfully for job ID 12345",
		"WARN: [WORKER] Task took longer than expected for job ID 11111",
		"ERROR: [WORKER] Task failed for job ID 22222: timeout error",
	},
}

func main() {
	common.Run(config)
}
