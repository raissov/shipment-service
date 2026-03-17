package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/raissov/shipment-service/internal/config"
)

// New creates a zerolog.Logger based on the provided config.
func New(cfg config.LogConfig) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	if cfg.Format == "console" {
		return zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).With().Timestamp().Logger()
	}

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
