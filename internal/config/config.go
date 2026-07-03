// Package config loads runtime configuration from the environment —
// twelve-factor style, no config files inside the image.
package config

import (
	"log/slog"
	"os"
	"time"
)

type Config struct {
	Port            string
	ShutdownTimeout time.Duration // how long in-flight requests may drain on SIGTERM
	LogLevel        slog.Level
}

func FromEnv() Config {
	cfg := Config{
		Port:            getenv("PORT", "8080"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		LogLevel:        getLevel("LOG_LEVEL", slog.LevelInfo),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getLevel(key string, fallback slog.Level) slog.Level {
	switch os.Getenv(key) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	default:
		return fallback
	}
}
