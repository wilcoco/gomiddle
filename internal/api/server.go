// Package api exposes the middleware's data over HTTP as JSON.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/wilcoco/gomiddle/internal/silo"
)

// New builds the HTTP server with all routes registered.
func New(addr string, poller *silo.Poller, log *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/silos", func(w http.ResponseWriter, r *http.Request) {
		snap := poller.Snapshot()
		status := http.StatusOK
		if snap.Error != "" {
			// The PLC is unreachable; the payload still carries the
			// last timestamp and the error so clients can decide.
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, snap)
	})

	return &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux, log),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// logRequests is a tiny middleware: it wraps every handler with an access log.
func logRequests(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
