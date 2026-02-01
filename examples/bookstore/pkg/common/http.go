package common

import (
	"fmt"
	"net/http"
)

func startHTTPServer(cfg Config) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	fmt.Printf("INFO: [%s] HTTP server listening on %s\n", cfg.Name, addr)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("ERROR: [%s] HTTP server error: %v\n", cfg.Name, err)
	}
}
