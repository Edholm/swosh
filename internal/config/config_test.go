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
				DatabaseURL:  "postgres://swosh:swosh@localhost:5432/swosh?sslmode=disable",
				ActuatorPort: 8081,
				LogLevel:     slog.LevelInfo,
			},
		},
		{
			name: "overrides",
			env: map[string]string{
				"SWOSH_DATABASE_URL":  "postgres://u:p@db:5432/swosh",
				"SWOSH_ACTUATOR_PORT": "9090",
				"LOG_LEVEL":           "debug",
			},
			want: Config{
				DatabaseURL:  "postgres://u:p@db:5432/swosh",
				ActuatorPort: 9090,
				LogLevel:     slog.LevelDebug,
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
