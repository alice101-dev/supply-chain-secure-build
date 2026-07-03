// Entrypoint: wire config, logging, handlers, and the HTTP server together,
// then run until SIGTERM with a graceful drain (matches the Kubernetes
// preStop / terminationGracePeriodSeconds contract).
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alice101-dev/supply-chain-secure-build/internal/config"
	"github.com/alice101-dev/supply-chain-secure-build/internal/handler"
	"github.com/alice101-dev/supply-chain-secure-build/internal/server"
)

// Stamped at build time via -ldflags. The same commit the SLSA provenance
// attests to is visible at runtime on /version — supply chain, end to end.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cfg := config.FromEnv()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	h := handler.New(log, version, commit)

	log.Info("starting server", "version", version, "commit", commit, "port", cfg.Port)
	if err := server.Run(context.Background(), cfg, h.Routes()); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("shutdown complete")
}
