package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := dbtest.URL(t)
	if err := db.Migrate(url, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

var (
	testNow     = time.Now()
	testSwoshes = []mongoSwosh{
		{ID: "active", Payee: "070-000 00 00", Amount: 100, ExpiresOn: ptr(testNow.Add(time.Hour)), CreatedAt: testNow},
		{ID: "never1", Payee: "1230000000", Amount: 1, CreatedAt: testNow},
		{ID: "old123", Payee: "p", Amount: 1, ExpiresOn: ptr(testNow.Add(-time.Hour)), CreatedAt: testNow},
	}
	testUsers = []mongoUser{
		{Username: "admin", Password: "{bcrypt}$2a$10$abc", Authorities: []mongoAuthority{{"ROLE_ADMIN"}}},
	}
)

func TestImportAll(t *testing.T) {
	pool := newPool(t)

	st, err := importAll(t.Context(), pool, testSwoshes, testUsers, testNow, false)
	if err != nil {
		t.Fatalf("importAll: %v", err)
	}
	if want := (stats{swoshInserted: 2, swoshExpired: 1, usersInserted: 1}); st != want {
		t.Errorf("first run stats = %+v, want %+v", st, want)
	}

	st, err = importAll(t.Context(), pool, testSwoshes, testUsers, testNow, false)
	if err != nil {
		t.Fatalf("importAll again: %v", err)
	}
	if want := (stats{swoshExisting: 2, swoshExpired: 1, usersExisting: 1}); st != want {
		t.Errorf("second run stats = %+v, want %+v", st, want)
	}

	if _, err := sqlc.New(pool).GetUserByUsername(t.Context(), "admin"); err != nil {
		t.Errorf("imported user not found: %v", err)
	}
}

func TestImportAllDryRunWritesNothing(t *testing.T) {
	pool := newPool(t)

	st, err := importAll(t.Context(), pool, testSwoshes, testUsers, testNow, true)
	if err != nil {
		t.Fatalf("importAll: %v", err)
	}
	if want := (stats{swoshInserted: 2, swoshExpired: 1, usersInserted: 1}); st != want {
		t.Errorf("dry run stats = %+v, want %+v", st, want)
	}

	list, err := sqlc.New(pool).ListSwoshes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("dry run left %d links in the database", len(list))
	}
}

func TestImportAllRollsBackOnError(t *testing.T) {
	pool := newPool(t)
	broken := []mongoSwosh{testSwoshes[0], {Payee: "no id"}}

	if _, err := importAll(t.Context(), pool, broken, nil, testNow, false); err == nil {
		t.Fatal("importAll with a broken document: got nil error")
	}
	list, err := sqlc.New(pool).ListSwoshes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("failed import left %d links in the database", len(list))
	}
}
