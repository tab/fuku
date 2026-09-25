package rest

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"fuku/internal/contracts"
)

func telemetryMiddleware(publisher contracts.Publisher, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		//nolint:errcheck // a non-critical publish never fails
		publisher.Publish(contracts.Message{
			Type: contracts.EventAPIRequested,
			Data: contracts.APIRequested{
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

// WriteHeader records the status code before passing it to the wrapped writer
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
		header := r.Header.Get("Authorization")

		if header == "" || len(header) < 7 || !strings.EqualFold(header[:7], "Bearer ") {
			writeError(w, http.StatusUnauthorized, ErrAPIUnauthorized)

			return
		}

		if subtle.ConstantTimeCompare([]byte(header[7:]), []byte(token)) != 1 {
			writeError(w, http.StatusUnauthorized, ErrAPIUnauthorized)

			return
		}

		next.ServeHTTP(w, r)
	})
}
