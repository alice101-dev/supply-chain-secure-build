// Package server owns the HTTP server lifecycle: hardened timeouts and
// graceful shutdown on SIGTERM/SIGINT.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/alice101-dev/supply-chain-secure-build/internal/config"
)

// readiness is the subset of the handler the server needs during shutdown.
type readiness interface{ SetReady(bool) }

// Run serves until SIGTERM/SIGINT, then drains in-flight requests for up to
// cfg.ShutdownTimeout. Keep that below terminationGracePeriodSeconds minus
// the preStop sleep, or Kubernetes will SIGKILL mid-drain.
func Run(ctx context.Context, cfg config.Config, h http.Handler) error {
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h,
		// Timeouts are load-shedding: without them one slow client
		// (Slowloris) can pin a connection forever.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		// Plain HTTP by design: in-cluster traffic; TLS terminates at the
		// ingress/mesh (see k8s/ — only same-namespace clients are allowed).
		// nosemgrep: go.lang.security.audit.net.use-tls.use-tls
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutdown signal received, draining", "timeout", cfg.ShutdownTimeout)
	if r, ok := h.(readiness); ok {
		r.SetReady(false) // readiness -> 503 so Kubernetes stops sending traffic
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(drainCtx)
}
