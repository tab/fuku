package shared

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
}

func Run(cfg Config) {
	fmt.Printf("INFO: [%s] Starting %s service...\n", cfg.Name, cfg.Name)
	time.Sleep(2 * time.Second)
	fmt.Printf("INFO: [%s] Service ready\n", cfg.Name)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	counter := 0
	msgCount := len(cfg.Messages)
	if msgCount == 0 {
		msgCount = 1
	}

	for {
		select {
		case <-sigCh:
			shutdown(cfg.Name)
			return
		case <-ticker.C:
			if len(cfg.Messages) > 0 {
				fmt.Printf("%s\n", cfg.Messages[counter%msgCount])
			} else {
				fmt.Printf("INFO: [%s] Processing at %s\n", cfg.Name, time.Now().Format(time.RFC3339))
			}
			counter++
		}
	}
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
