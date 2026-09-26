package rest

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"fuku/internal/contracts"
)

// bearerPrefix is the scheme prefix an Authorization header must carry
const bearerPrefix = "Bearer "

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
				Route:    route(r.Pattern),
				Status:   rw.status,
				Duration: time.Since(start),
			},
		})
	})
}

// route strips the method from a ServeMux pattern, leaving the templated path
func route(pattern string) string {
	_, path, found := strings.Cut(pattern, " ")
	if !found {
		return pattern
	}

	return path
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

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
			writeError(w, http.StatusUnauthorized, ErrAPIUnauthorized)

			return
		}

		if subtle.ConstantTimeCompare([]byte(header[len(bearerPrefix):]), []byte(token)) != 1 {
			writeError(w, http.StatusUnauthorized, ErrAPIUnauthorized)

			return
		}

		next.ServeHTTP(w, r)
	})
}
