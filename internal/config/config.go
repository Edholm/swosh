// Package config loads swosh's settings from environment variables.
package config

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sethvargo/go-envconfig"
)

// Config holds the settings for the swosh server. Variable names match those
// of the Kotlin service where the setting still exists.
type Config struct {
	// DatabaseURL is a postgres:// URL. The default matches docker-compose.dev.yml.
	DatabaseURL string `env:"SWOSH_DATABASE_URL, default=postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable"`

	// ActuatorPort serves operational endpoints such as /health.
	ActuatorPort int `env:"SWOSH_ACTUATOR_PORT, default=8081"`

	// LogLevel is one of debug, info, warn or error.
	LogLevel slog.Level `env:"LOG_LEVEL, default=info"`
}

// Load reads the configuration from the process environment.
func Load(ctx context.Context) (*Config, error) {
	return LoadFrom(ctx, envconfig.OsLookuper())
}

// LoadFrom reads the configuration from l.
func LoadFrom(ctx context.Context, l envconfig.Lookuper) (*Config, error) {
	var cfg Config
	if err := envconfig.ProcessWith(ctx, &envconfig.Config{Target: &cfg, Lookuper: l}); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &cfg, nil
}
