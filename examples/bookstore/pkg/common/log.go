package common

import (
	"fmt"
	"time"
)

func logMessages(cfg Config, stopCh <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	counter := 0
	msgCount := len(cfg.Messages)
	if msgCount == 0 {
		msgCount = 1
	}

	for {
		select {
		case <-stopCh:
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
