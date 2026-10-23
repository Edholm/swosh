package auth

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

func TestCheckPassword(t *testing.T) {
	// "admin", as Spring Security stored it for the Kotlin service.
	const spring = "{bcrypt}$2a$10$0MfTyXDYOZTLPDoT7hqrr.xH1qI8A8s/DmBMp1R94iNznJsP0r.6y"
	raw, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, stored, password string
		ok                     bool
	}{
		{"spring hash", spring, "admin", true},
		{"wrong password", spring, "Admin", false},
		{"no prefix", string(raw), "admin", false},
		{"noop encoder", "{noop}admin", "admin", false},
		{"other encoder", "{scrypt}" + string(raw), "admin", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.stored, tt.password)
			if (err == nil) != tt.ok {
				t.Errorf("CheckPassword = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "{bcrypt}$2a$10$") {
		t.Errorf("hash = %q, want Spring's {bcrypt}$2a$10$ format", h)
	}
	if err := CheckPassword(h, "hunter2"); err != nil {
		t.Errorf("CheckPassword of own hash: %v", err)
	}
}

func TestSessions(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := NewSigner([]byte("0123456789abcdef0123456789abcdef"))
	sess := s.New("admin", "{bcrypt}hash", now.Add(time.Hour))
	cookie := s.Encode(sess)

	got, err := s.Decode(cookie, now)
	if err != nil || got != sess {
		t.Fatalf("Decode = %+v, %v; want %+v", got, err, sess)
	}
	if got.Password != s.PasswordTag("{bcrypt}hash") || got.Password == s.PasswordTag("{bcrypt}other") {
		t.Error("PasswordTag does not tell password hashes apart")
	}

	payload, sig, _ := strings.Cut(cookie, ".")
	forged := s.Encode(Session{Username: "root", Expires: sess.Expires, Nonce: sess.Nonce})
	forgedPayload, _, _ := strings.Cut(forged, ".")
	for name, value := range map[string]string{
		"expired":         "",
		"other key":       NewSigner(RandomKey()).Encode(sess),
		"swapped payload": forgedPayload + "." + sig,
		"no signature":    payload,
		"garbage":         "!!!.???",
	} {
		at := now
		if name == "expired" {
			value, at = cookie, now.Add(time.Hour)
		}
		if _, err := s.Decode(value, at); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("%s: Decode err = %v, want ErrInvalidSession", name, err)
		}
	}

	token := s.CSRFToken(sess)
	if !s.CheckCSRFToken(sess, token) {
		t.Error("CSRF token of the session is rejected")
	}
	other := s.New("admin", "{bcrypt}hash", now.Add(time.Hour))
	if s.CheckCSRFToken(other, token) || s.CheckCSRFToken(sess, "") {
		t.Error("CSRF token is accepted for another session, or an empty one is")
	}
}

type fakeUsers struct {
	users   map[string]sqlc.User
	err     error
	created []sqlc.CreateUserParams
}

func (f *fakeUsers) GetUserByUsername(_ context.Context, name string) (sqlc.User, error) {
	if f.err != nil {
		return sqlc.User{}, f.err
	}
	u, ok := f.users[name]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (f *fakeUsers) CreateUser(_ context.Context, arg sqlc.CreateUserParams) (int64, error) {
	f.created = append(f.created, arg)
	return 1, f.err
}

func TestLoginDatabaseError(t *testing.T) {
	f := &fakeUsers{err: errors.New("connection refused")}
	if _, err := Login(t.Context(), f, "admin", "admin"); err == nil || errors.Is(err, ErrBadCredentials) {
		t.Errorf("Login err = %v, want a database error", err)
	}
}

func TestProvisionWarnsAboutDefaultPassword(t *testing.T) {
	for _, tt := range []struct {
		password string
		warn     bool
	}{{"admin", true}, {"hunter2", false}} {
		var buf bytes.Buffer
		log := slog.New(slog.NewTextHandler(&buf, nil))
		f := &fakeUsers{}
		if err := Provision(t.Context(), f, "admin", tt.password, log); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(buf.String(), "level=WARN"); got != tt.warn {
			t.Errorf("password %q: warned = %v, want %v", tt.password, got, tt.warn)
		}
		if len(f.created) != 1 || strings.Join(f.created[0].Authorities, ",") != "ROLE_USER,ROLE_ADMIN,DELETE,UPDATE" {
			t.Errorf("created %+v", f.created)
		}
	}
}

func newQueries(t *testing.T) *sqlc.Queries {
	t.Helper()
	url := dbtest.URL(t)
	if err := db.Migrate(url, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return sqlc.New(pool)
}

func TestProvisionAndLoginWithPostgres(t *testing.T) {
	q := newQueries(t)
	log := slog.New(slog.DiscardHandler)
	ctx := t.Context()

	if err := Provision(ctx, q, "admin", "first", log); err != nil {
		t.Fatal(err)
	}
	// An existing user keeps its password.
	if err := Provision(ctx, q, "admin", "second", log); err != nil {
		t.Fatal(err)
	}
	u, err := Login(ctx, q, "admin", "first")
	if err != nil {
		t.Fatalf("Login with the first password: %v", err)
	}
	if !strings.HasPrefix(u.Password, "{bcrypt}") || strings.Join(u.Authorities, ",") != "ROLE_USER,ROLE_ADMIN,DELETE,UPDATE" {
		t.Errorf("user = %+v", u)
	}
	if _, err := Login(ctx, q, "admin", "second"); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("Login with the second password: err = %v, want ErrBadCredentials", err)
	}
	if _, err := Login(ctx, q, "nobody", "first"); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("Login as unknown user: err = %v, want ErrBadCredentials", err)
	}

	hash, err := HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "off", Password: hash, Enabled: false, Authorities: AdminAuthorities}); err != nil {
		t.Fatal(err)
	}
	if _, err := Login(ctx, q, "off", "pw"); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("Login as disabled user: err = %v, want ErrBadCredentials", err)
	}
}
