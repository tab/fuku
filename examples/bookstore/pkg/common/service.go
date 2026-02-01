package common

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Name     string
	Messages []string
	HTTPPort int
	TCPPort  int
}

func Run(cfg Config) {
	fmt.Printf("INFO: [%s] run at %s\n", cfg.Name, time.Now().Format(time.RFC3339))
	fmt.Printf("INFO: [%s] Starting %s service...\n", cfg.Name, cfg.Name)

	if cfg.HTTPPort > 0 {
		go startHTTPServer(cfg)
	}

	if cfg.TCPPort > 0 {
		go startTCPServer(cfg)
	}

	time.Sleep(2 * time.Second)
	fmt.Printf("INFO: [%s] Service ready\n", cfg.Name)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	stopCh := make(chan struct{})

	go logMessages(cfg, stopCh)

	<-sigCh
	close(stopCh)
	shutdown(cfg.Name)
}

func shutdown(name string) {
	fmt.Printf("INFO: [%s] Received shutdown signal, starting graceful shutdown...\n", name)
	fmt.Printf("INFO: [%s] Draining active connections...\n", name)
	time.Sleep(300 * time.Millisecond)
	fmt.Printf("INFO: [%s] Closing database connections...\n", name)
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("INFO: [%s] Flushing caches...\n", name)
	time.Sleep(200 * time.Millisecond)
	fmt.Printf("INFO: [%s] Shutdown complete, goodbye!\n", name)
}
