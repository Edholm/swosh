package config

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestLoadFrom(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "defaults",
			env:  nil,
			want: Config{
				DatabaseURL:      "postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable",
				ServerPort:       8080,
				ActuatorPort:     8081,
				LogLevel:         slog.LevelInfo,
				Scheme:           "https",
				Hostname:         "swosh.me",
				ExpireRateMillis: 60000,
				SwishQREndpoint:  "https://mpc.getswish.net/qrg-swish/api/v1/prefilled",
				Environment:      "local",
				AppName:          "swosh",
			},
		},
		{
			name: "overrides",
			env: map[string]string{
				"SWOSH_DATABASE_URL":      "postgres://u:p@db:5432/swosh",
				"SERVER_PORT":             "9080",
				"SWOSH_ACTUATOR_PORT":     "9090",
				"LOG_LEVEL":               "debug",
				"SWOSH_SCHEME":            "http",
				"SWOSH_HOSTNAME":          "localhost:9080",
				"SWOSH_EXPIRE_RATE":       "1000",
				"SWOSH_SWISH_QR_ENDPOINT": "http://qr.test/prefilled",
				"SWOSH_ENVIRONMENT":       "prod",
				"SWOSH_SERVER_HOSTNAME":   "web1",
				"SWOSH_APP_NAME":          "swosh-go",
			},
			want: Config{
				DatabaseURL:      "postgres://u:p@db:5432/swosh",
				ServerPort:       9080,
				ActuatorPort:     9090,
				LogLevel:         slog.LevelDebug,
				Scheme:           "http",
				Hostname:         "localhost:9080",
				ExpireRateMillis: 1000,
				SwishQREndpoint:  "http://qr.test/prefilled",
				Environment:      "prod",
				ServerHostname:   "web1",
				AppName:          "swosh-go",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFrom(t.Context(), envconfig.MapLookuper(tt.env))
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if *got != tt.want {
				t.Errorf("LoadFrom = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestLoadFromInvalid(t *testing.T) {
	tests := []struct {
		env, value, field string
	}{
		{"SWOSH_ACTUATOR_PORT", "eighty", "ActuatorPort"},
		{"SWOSH_EXPIRE_RATE", "0", "ExpireRateMillis"},
		// The Kotlin service defaulted to TRACE, which slog does not have.
		{"LOG_LEVEL", "trace", "LogLevel"},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			env := envconfig.MapLookuper(map[string]string{tt.env: tt.value})
			_, err := LoadFrom(t.Context(), env)
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Errorf("LoadFrom with %s=%q: err = %v, want an error naming %s", tt.env, tt.value, err, tt.field)
			}
		})
	}
}
