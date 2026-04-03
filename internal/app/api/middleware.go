package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"fuku/internal/app/errors"
)

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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIUnauthorized.Error()})

			return
		}

		if header[7:] != token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)

			//nolint:errcheck // best-effort JSON encoding
			json.NewEncoder(w).Encode(ErrorSerializer{Error: errors.ErrAPIUnauthorized.Error()})

			return
		}

		next.ServeHTTP(w, r)
	})
}
