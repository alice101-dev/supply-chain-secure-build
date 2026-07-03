// Package handler owns the HTTP surface: routes, probes, and request logging.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

type Handler struct {
	log     *slog.Logger
	version string
	commit  string
	ready   atomic.Bool
}

func New(log *slog.Logger, version, commit string) *Handler {
	h := &Handler{log: log, version: version, commit: commit}
	h.ready.Store(true)
	return h
}

// SetReady flips the readiness gate — server.Run marks the service unready
// at the start of a graceful shutdown so Kubernetes drains traffic first.
func (h *Handler) SetReady(ready bool) { h.ready.Store(ready) }

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness: process is up. Kept trivial on purpose — a failing liveness
	// probe restarts the pod, so it must never depend on downstream systems.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Readiness: willing to accept traffic. Flips to 503 during shutdown.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !h.ready.Load() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// The build identity, verifiable against the image's SLSA provenance.
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"version": h.version,
			"commit":  h.commit,
		})
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "hello from a signed, attested, SBOM-carrying image 📦🔏",
		})
	})

	return h.requestLogger(mux)
}

// requestLogger logs one structured line per request — skipping the probe
// endpoints, which would otherwise drown the logs at scrape frequency.
func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already written; nothing left to do but log.
		slog.Error("encoding response", "error", err)
	}
}
