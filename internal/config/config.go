// Package config loads swosh's settings from environment variables.
package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds the settings for the swosh server. Variable names match those
// of the Kotlin service where the setting still exists.
type Config struct {
	// DatabaseURL is a postgres:// URL. The default matches docker-compose.dev.yml.
	DatabaseURL string `env:"SWOSH_DATABASE_URL, default=postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable"`

	// ServerPort serves the public site and API.
	ServerPort int `env:"SERVER_PORT, default=8080"`

	// ActuatorPort serves operational endpoints such as /health and /prometheus.
	ActuatorPort int `env:"SWOSH_ACTUATOR_PORT, default=8081"`

	// LogLevel is one of debug, info, warn or error.
	LogLevel slog.Level `env:"LOG_LEVEL, default=info"`

	// Scheme and Hostname form the public URL of created links.
	Scheme   string `env:"SWOSH_SCHEME, default=https"`
	Hostname string `env:"SWOSH_HOSTNAME, default=swosh.me"`

	// ExpireRateMillis is how often expired links are deleted, in
	// milliseconds as in the Kotlin service.
	ExpireRateMillis int `env:"SWOSH_EXPIRE_RATE, default=60000"`

	// SwishQREndpoint is Swish's API for prefilled QR codes.
	SwishQREndpoint string `env:"SWOSH_SWISH_QR_ENDPOINT, default=https://mpc.getswish.net/qrg-swish/api/v1/prefilled"`

	// Environment, ServerHostname and AppName label every metric.
	// ServerHostname defaults to the machine's hostname.
	Environment    string `env:"SWOSH_ENVIRONMENT, default=local"`
	ServerHostname string `env:"SWOSH_SERVER_HOSTNAME"`
	AppName        string `env:"SWOSH_APP_NAME, default=swosh"`
}

// ExpireRate is ExpireRateMillis as a duration.
func (c *Config) ExpireRate() time.Duration {
	return time.Duration(c.ExpireRateMillis) * time.Millisecond
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
	if cfg.ExpireRateMillis <= 0 {
		return nil, fmt.Errorf("load config: ExpireRateMillis: SWOSH_EXPIRE_RATE must be positive, got %d", cfg.ExpireRateMillis)
	}
	return &cfg, nil
}
