package main

import (
	"examples/bookstore/pkg/common"
)

var config = common.Config{
	Name: "AUTH",
	Messages: []string{
		"INFO: [AUTH] Token validated for user_123",
		"DEBUG: [AUTH] JWT signature verified",
		"INFO: [AUTH] New session created",
		"WARN: [AUTH] Rate limit warning for IP 192.168.1.1",
	},
}

func main() {
	common.Run(config)
}
