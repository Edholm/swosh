// Package dbtest gives tests an isolated PostgreSQL schema to work in.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// EnvVar names the environment variable with the URL of a Postgres database
// that tests may create and drop schemas in.
const EnvVar = "SWOSH_TEST_DATABASE_URL"

// URL returns a connection URL whose search_path points at a new, empty
// schema. The schema is dropped when the test ends. The test is skipped when
// SWOSH_TEST_DATABASE_URL is not set.
func URL(tb testing.TB) string {
	tb.Helper()
	base := os.Getenv(EnvVar)
	if base == "" {
		tb.Skipf("%s is not set; skipping database test", EnvVar)
	}

	u, err := url.Parse(base)
	if err != nil {
		tb.Fatalf("%s is not a valid URL", EnvVar)
	}
	schema := "test_" + randomHex(tb)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	ctx := tb.Context()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		tb.Fatalf("connect to test database: %v", err)
	}
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		tb.Fatalf("create schema %s: %v", schema, err)
	}
	tb.Cleanup(func() {
		// tb.Context is already cancelled when cleanups run.
		ctx := context.Background()
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			tb.Errorf("drop schema %s: %v", schema, err)
		}
		conn.Close(ctx)
	})
	return u.String()
}

func randomHex(tb testing.TB) string {
	tb.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		tb.Fatal(err)
	}
	return hex.EncodeToString(b)
}
