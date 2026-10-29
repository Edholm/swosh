package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sethvargo/go-envconfig"

	"github.com/Edholm/swosh/internal/auth"
	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

func run(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	return runWithInput(t, env, "", args...)
}

func runWithInput(t *testing.T, env map[string]string, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := NewRootCmd(envconfig.MapLookuper(env))
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
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
		srv := httptest.NewServer(healthHandler(fakePinger{pingErr}))
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

func TestAdminAddRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		args    []string
		wantErr string
	}{
		{"no username", "pw\n", []string{"admin", "add"}, "accepts 1 arg"},
		{"empty username", "pw\n", []string{"admin", "add", ""}, "username"},
		{"padded username", "pw\n", []string{"admin", "add", " alice"}, "username"},
		{"no password", "", []string{"admin", "add", "alice"}, "password must not be empty"},
		{"empty line", "\n", []string{"admin", "add", "alice"}, "password must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The database is never reached, so the default URL does not matter.
			_, err := runWithInput(t, nil, tt.stdin, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestAdminAdd(t *testing.T) {
	url := dbtest.URL(t)
	if err := db.Migrate(url); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"SWOSH_DATABASE_URL": url}

	out, err := runWithInput(t, env, "hunter2\nignored\n", "admin", "add", "alice")
	if err != nil {
		t.Fatalf("admin add alice: %v", err)
	}
	if want := "Created admin user \"alice\"\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
	if _, err := runWithInput(t, env, "s3cret\r\n", "admin", "add", "bob"); err != nil {
		t.Fatalf("admin add bob: %v", err)
	}
	if _, err := runWithInput(t, env, "other\n", "admin", "add", "alice"); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("admin add alice again: err = %v, want ErrUserExists", err)
	}

	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)
	for name, password := range map[string]string{"alice": "hunter2", "bob": "s3cret"} {
		u, err := auth.Login(t.Context(), q, name, password)
		if err != nil {
			t.Errorf("Login as %s: %v", name, err)
			continue
		}
		if !slices.Equal(u.Authorities, auth.AdminAuthorities) {
			t.Errorf("%s has authorities %v, want %v", name, u.Authorities, auth.AdminAuthorities)
		}
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
			h := healthHandler(fakePinger{tt.pingErr})
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
