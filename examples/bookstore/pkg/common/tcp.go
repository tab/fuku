package common

import (
	"fmt"
	"net"
	"time"
)

func startTCPServer(cfg Config) {
	addr := fmt.Sprintf(":%d", cfg.TCPPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("ERROR: [%s] TCP listen error: %v\n", cfg.Name, err)
		return
	}

	fmt.Printf("INFO: [%s] TCP server listening on %s (gRPC)\n", cfg.Name, addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			time.Sleep(100 * time.Millisecond)
		}(conn)
	}
}
