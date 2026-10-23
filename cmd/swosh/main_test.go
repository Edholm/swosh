package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func run(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := NewRootCmd(envconfig.MapLookuper(env))
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestVersion(t *testing.T) {
	// A broken environment must not stop the version from printing.
	out, err := run(t, map[string]string{"SWOSH_ACTUATOR_PORT": "nope"}, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if out != version+"\n" {
		t.Errorf("version printed %q, want %q", out, version+"\n")
	}
}

func TestUnknownCommand(t *testing.T) {
	if _, err := run(t, nil, "frobnicate"); err == nil {
		t.Error("unknown command: got nil error")
	}
}

func TestServeRejectsInvalidConfig(t *testing.T) {
	_, err := run(t, map[string]string{"SWOSH_ACTUATOR_PORT": "nope"}, "serve")
	if err == nil || !strings.Contains(err.Error(), "ActuatorPort") {
		t.Errorf("serve with invalid port: err = %v, want a config error naming ActuatorPort", err)
	}
}

func TestHealthcheck(t *testing.T) {
	// actuatorPort serves healthHandler with a database that fails with
	// pingErr and returns its port. With closed set, nothing listens on it.
	actuatorPort := func(t *testing.T, pingErr error, closed bool) string {
		t.Helper()
		srv := httptest.NewServer(healthHandler(fakePinger{pingErr}, slog.New(slog.DiscardHandler)))
		port := strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
		if closed {
			srv.Close()
		} else {
			t.Cleanup(srv.Close)
		}
		return port
	}

	tests := []struct {
		name    string
		pingErr error
		closed  bool
		wantErr string
	}{
		{"up", nil, false, ""},
		{"database down", errors.New("connection refused"), false, "503"},
		{"server not running", nil, true, "health check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := actuatorPort(t, tt.pingErr, tt.closed)
			_, err := run(t, map[string]string{"SWOSH_ACTUATOR_PORT": port}, "healthcheck")
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("healthcheck: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("healthcheck: err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantCode   int
		wantStatus string
	}{
		{"database up", nil, http.StatusOK, "UP"},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, "DOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := healthHandler(fakePinger{tt.pingErr}, slog.New(slog.DiscardHandler))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

			if rec.Code != tt.wantCode {
				t.Errorf("status code = %d, want %d", rec.Code, tt.wantCode)
			}
			var body struct{ Status string }
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body, err)
			}
			if body.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", body.Status, tt.wantStatus)
			}
		})
	}
}
