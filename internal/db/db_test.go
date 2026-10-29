package db

import (
	"errors"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

// newQueries migrates a fresh schema and returns queries bound to it.
func newQueries(t *testing.T) *sqlc.Queries {
	t.Helper()
	url := dbtest.URL(t)
	if err := Migrate(url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := Connect(t.Context(), url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return sqlc.New(pool)
}

// latestVersion is the highest migration version embedded in the binary.
func latestVersion(t *testing.T) uint {
	t.Helper()
	files, err := fs.Glob(migrations, "migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no embedded migrations: %v", err)
	}
	name := strings.TrimPrefix(slices.Max(files), "migrations/")
	v, err := strconv.ParseUint(name[:strings.IndexByte(name, '_')], 10, 0)
	if err != nil {
		t.Fatalf("parse version of %s: %v", name, err)
	}
	return uint(v)
}

func schemaVersionOf(t *testing.T, url string) uint {
	t.Helper()
	m, err := newMigrator(url)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	v, err := schemaVersion(m)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMigrateIsIdempotent(t *testing.T) {
	url := dbtest.URL(t)
	for range 2 {
		if err := Migrate(url); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
	}
	if got, want := schemaVersionOf(t, url), latestVersion(t); got != want {
		t.Errorf("schema version = %d, want %d", got, want)
	}
}

func TestMigrateDownAndUp(t *testing.T) {
	url := dbtest.URL(t)
	if err := Migrate(url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	m, err := newMigrator(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
	m.Close()
	if got := schemaVersionOf(t, url); got != 0 {
		t.Fatalf("schema version after Down = %d, want 0", got)
	}

	if err := Migrate(url); err != nil {
		t.Fatalf("Migrate after Down: %v", err)
	}
}

func TestMigrateRejectsNonPostgresURL(t *testing.T) {
	err := Migrate("mysql://root:secret@localhost/swosh")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("Migrate(mysql://…) = %v, want an error without the password", err)
	}
}

func TestCreateAndGetSwosh(t *testing.T) {
	q := newQueries(t)
	ctx := t.Context()
	before := time.Now()

	desc := "Pizza"
	expires := time.Now().Add(48 * time.Hour).Truncate(time.Microsecond)
	created, err := q.CreateSwosh(ctx, sqlc.CreateSwoshParams{
		ID: "abc123", Payee: "070-000 00 00", Amount: 100, Description: &desc, ExpiresOn: &expires,
	})
	if err != nil {
		t.Fatalf("CreateSwosh: %v", err)
	}
	if created.CreatedAt.Before(before.Add(-time.Minute)) {
		t.Errorf("CreatedAt = %v, want about now", created.CreatedAt)
	}

	got, err := q.GetSwosh(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetSwosh: %v", err)
	}
	if got.Payee != "070-000 00 00" || got.Amount != 100 || *got.Description != desc || !got.ExpiresOn.Equal(expires) {
		t.Errorf("GetSwosh = %+v, want the created link", got)
	}

	if _, err := q.GetSwosh(ctx, "nope"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetSwosh(missing) error = %v, want pgx.ErrNoRows", err)
	}
}

func TestCreateSwoshWithoutOptionalFields(t *testing.T) {
	q := newQueries(t)
	if _, err := q.CreateSwosh(t.Context(), sqlc.CreateSwoshParams{ID: "plain1", Payee: "1230000000", Amount: 1}); err != nil {
		t.Fatalf("CreateSwosh: %v", err)
	}
	got, err := q.GetSwosh(t.Context(), "plain1")
	if err != nil {
		t.Fatalf("GetSwosh: %v", err)
	}
	if got.Description != nil || got.ExpiresOn != nil {
		t.Errorf("Description = %v, ExpiresOn = %v, want both nil", got.Description, got.ExpiresOn)
	}
}

func TestListSwoshesOrdersByExpiryWithNeverExpiringLast(t *testing.T) {
	q := newQueries(t)
	soon := time.Now().Add(time.Hour)
	later := time.Now().Add(2 * time.Hour)
	for _, p := range []sqlc.CreateSwoshParams{
		{ID: "soon", Payee: "p", Amount: 1, ExpiresOn: &soon},
		{ID: "never", Payee: "p", Amount: 1},
		{ID: "later", Payee: "p", Amount: 1, ExpiresOn: &later},
	} {
		if _, err := q.CreateSwosh(t.Context(), p); err != nil {
			t.Fatalf("CreateSwosh(%s): %v", p.ID, err)
		}
	}

	list, err := q.ListSwoshes(t.Context())
	if err != nil {
		t.Fatalf("ListSwoshes: %v", err)
	}
	var ids []string
	for _, s := range list {
		ids = append(ids, s.ID)
	}
	if want := []string{"later", "soon", "never"}; !slices.Equal(ids, want) {
		t.Errorf("ListSwoshes order = %v, want %v", ids, want)
	}
}

func TestUpdateSwoshRenames(t *testing.T) {
	q := newQueries(t)
	ctx := t.Context()
	if _, err := q.CreateSwosh(ctx, sqlc.CreateSwoshParams{ID: "old", Payee: "p", Amount: 1}); err != nil {
		t.Fatal(err)
	}

	desc := "renamed"
	n, err := q.UpdateSwosh(ctx, sqlc.UpdateSwoshParams{ID: "old", NewID: "new", Payee: "q", Amount: 5, Description: &desc})
	if err != nil || n != 1 {
		t.Fatalf("UpdateSwosh = %d, %v; want 1 row", n, err)
	}
	if _, err := q.GetSwosh(ctx, "old"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetSwosh(old) error = %v, want pgx.ErrNoRows", err)
	}
	got, err := q.GetSwosh(ctx, "new")
	if err != nil {
		t.Fatalf("GetSwosh(new): %v", err)
	}
	if got.Payee != "q" || got.Amount != 5 || *got.Description != desc {
		t.Errorf("GetSwosh(new) = %+v, want updated fields", got)
	}

	if n, err := q.UpdateSwosh(ctx, sqlc.UpdateSwoshParams{ID: "missing", NewID: "x", Payee: "p", Amount: 1}); err != nil || n != 0 {
		t.Errorf("UpdateSwosh(missing) = %d, %v; want 0 rows", n, err)
	}
}

func TestDeleteSwosh(t *testing.T) {
	q := newQueries(t)
	if _, err := q.CreateSwosh(t.Context(), sqlc.CreateSwoshParams{ID: "gone", Payee: "p", Amount: 1}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int64{1, 0} {
		if n, err := q.DeleteSwosh(t.Context(), "gone"); err != nil || n != want {
			t.Errorf("DeleteSwosh = %d, %v; want %d rows", n, err, want)
		}
	}
}

func TestDeleteExpiredSwoshes(t *testing.T) {
	q := newQueries(t)
	now := time.Now()
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	for _, p := range []sqlc.CreateSwoshParams{
		{ID: "expired", Payee: "p", Amount: 1, ExpiresOn: &past},
		{ID: "active", Payee: "p", Amount: 1, ExpiresOn: &future},
		{ID: "never", Payee: "p", Amount: 1},
	} {
		if _, err := q.CreateSwosh(t.Context(), p); err != nil {
			t.Fatal(err)
		}
	}

	ids, err := q.DeleteExpiredSwoshes(t.Context(), now)
	if err != nil || !slices.Equal(ids, []string{"expired"}) {
		t.Fatalf("DeleteExpiredSwoshes = %v, %v; want [expired]", ids, err)
	}
	if _, err := q.GetSwosh(t.Context(), "expired"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expired link still exists: %v", err)
	}
	if list, _ := q.ListSwoshes(t.Context()); len(list) != 2 {
		t.Errorf("%d links left, want 2", len(list))
	}
}

func TestImportSwoshKeepsExistingRow(t *testing.T) {
	q := newQueries(t)
	ctx := t.Context()
	createdAt := time.Date(2020, 4, 1, 12, 0, 0, 123456000, time.UTC)

	first := sqlc.ImportSwoshParams{ID: "imp", Payee: "first", Amount: 1, CreatedAt: createdAt}
	if n, err := q.ImportSwosh(ctx, first); err != nil || n != 1 {
		t.Fatalf("ImportSwosh = %d, %v; want 1 row", n, err)
	}
	second := sqlc.ImportSwoshParams{ID: "imp", Payee: "second", Amount: 2, CreatedAt: time.Now()}
	if n, err := q.ImportSwosh(ctx, second); err != nil || n != 0 {
		t.Fatalf("ImportSwosh(duplicate) = %d, %v; want 0 rows", n, err)
	}

	got, err := q.GetSwosh(ctx, "imp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Payee != "first" || !got.CreatedAt.Equal(createdAt) {
		t.Errorf("GetSwosh = %+v, want the first import with CreatedAt %v", got, createdAt)
	}
}

func TestCreateUser(t *testing.T) {
	q := newQueries(t)
	ctx := t.Context()
	params := sqlc.CreateUserParams{
		Username:    "admin",
		Password:    "{bcrypt}$2a$10$abcdefghijklmnopqrstuv",
		Enabled:     true,
		Authorities: []string{"ROLE_USER", "ROLE_ADMIN", "DELETE", "UPDATE"},
	}
	if n, err := q.CreateUser(ctx, params); err != nil || n != 1 {
		t.Fatalf("CreateUser = %d, %v; want 1 row", n, err)
	}
	if n, err := q.CreateUser(ctx, params); err != nil || n != 0 {
		t.Fatalf("CreateUser(duplicate) = %d, %v; want 0 rows", n, err)
	}

	got, err := q.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != params.Password || !got.Enabled || !slices.Equal(got.Authorities, params.Authorities) {
		t.Errorf("GetUserByUsername = %+v, want %+v", got, params)
	}
}
