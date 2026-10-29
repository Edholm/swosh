// Package auth checks passwords stored in Spring Security's format, signs
// login sessions and creates admin users.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/Edholm/swosh/internal/db/sqlc"
)

// Authorities, as Spring Security named them.
const (
	RoleAdmin = "ROLE_ADMIN"
	Update    = "UPDATE"
	Delete    = "DELETE"
)

// AdminAuthorities are what a provisioned user gets, as in the Kotlin service.
var AdminAuthorities = []string{"ROLE_USER", RoleAdmin, Delete, Update}

const bcryptPrefix = "{bcrypt}"

// ErrBadCredentials means the username or password is wrong, or the user is
// disabled. Callers should not tell the cases apart to the client.
var ErrBadCredentials = errors.New("bad credentials")

// HashPassword hashes password with bcrypt in the "{bcrypt}$2a$…" format of
// Spring's DelegatingPasswordEncoder.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return bcryptPrefix + string(h), nil
}

// CheckPassword reports whether password matches stored, which must be in
// the "{bcrypt}…" format. Other encoders Spring supported ({noop}, {scrypt},
// …) and hashes without an {id} prefix are rejected.
func CheckPassword(stored, password string) error {
	hash, ok := strings.CutPrefix(stored, bcryptPrefix)
	if !ok {
		return ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrBadCredentials
	}
	return nil
}

// dummyHash is compared against when the user does not exist, so that a
// missing user takes as long to reject as a wrong password.
var dummyHash = func() string {
	h, err := bcrypt.GenerateFromPassword([]byte("not a password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return bcryptPrefix + string(h)
}()

// UserGetter looks users up by name.
type UserGetter interface {
	GetUserByUsername(ctx context.Context, username string) (sqlc.User, error)
}

// Login returns the user if username and password match an enabled user.
// It returns ErrBadCredentials if they don't, and other errors if the
// database fails.
func Login(ctx context.Context, users UserGetter, username, password string) (sqlc.User, error) {
	u, err := users.GetUserByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, fmt.Errorf("get user: %w", err)
		}
		_ = CheckPassword(dummyHash, password)
		return sqlc.User{}, ErrBadCredentials
	}
	if err := CheckPassword(u.Password, password); err != nil {
		return sqlc.User{}, err
	}
	if !u.Enabled {
		return sqlc.User{}, ErrBadCredentials
	}
	return u, nil
}

// UserCreator inserts users, leaving existing ones alone.
type UserCreator interface {
	CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (int64, error)
}

// ErrUserExists means a user with the username already exists.
var ErrUserExists = errors.New("user already exists")

// CreateAdmin creates an enabled user with AdminAuthorities. If the username
// is taken, it returns ErrUserExists and leaves that user as it is.
func CreateAdmin(ctx context.Context, users UserCreator, username, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	n, err := users.CreateUser(ctx, sqlc.CreateUserParams{
		Username:    username,
		Password:    hash,
		Enabled:     true,
		Authorities: AdminAuthorities,
	})
	if err != nil {
		return fmt.Errorf("create user %q: %w", username, err)
	}
	if n == 0 {
		return fmt.Errorf("create user %q: %w", username, ErrUserExists)
	}
	return nil
}

// Provision creates an admin user with the given credentials unless a user
// with that name exists. An existing user's password and authorities are
// left as they are.
func Provision(ctx context.Context, users UserCreator, username, password string) error {
	if password == "admin" {
		slog.WarnContext(ctx, "the admin user is provisioned with the default password; set SWOSH_ADMIN_PWD", slog.String("username", username))
	}
	err := CreateAdmin(ctx, users, username, password)
	switch {
	case errors.Is(err, ErrUserExists):
		slog.DebugContext(ctx, "admin user already exists", slog.String("username", username))
	case err != nil:
		return fmt.Errorf("provision admin user: %w", err)
	default:
		slog.InfoContext(ctx, "provisioned admin user", slog.String("username", username))
	}
	return nil
}
