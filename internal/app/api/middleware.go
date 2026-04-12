package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"fuku/internal/app/bus"
	"fuku/internal/app/errors"
)

func telemetryMiddleware(b bus.Bus, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		b.Publish(bus.Message{
			Type: bus.EventAPIRequest,
			Data: bus.APIRequest{
				Method:   r.Method,
				Path:     r.URL.Path,
				Status:   rw.status,
				Duration: time.Since(start),
			},
		})
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIUnauthorized.Error()})

			return
		}

		header := r.Header.Get("Authorization")

		if header == "" || len(header) < 7 || !strings.EqualFold(header[:7], "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIUnauthorized.Error()})

			return
		}

		if subtle.ConstantTimeCompare([]byte(header[7:]), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIUnauthorized.Error()})

			return
		}

		next.ServeHTTP(w, r)
	})
}
