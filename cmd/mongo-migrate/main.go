// Command mongo-migrate copies swosh links and admin users from the MongoDB
// database of the old Kotlin service into PostgreSQL.
//
// Links that have already expired are skipped. Rows that already exist in
// Postgres are left untouched, so the command can be run again, e.g. once
// against staging and then right before switching production over. Spring
// sessions are not copied; admins have to log in again.
//
// Usage:
//
//	SWOSH_MONGODB_URI=mongodb://mongodb/swosh \
//	SWOSH_DATABASE_URL=postgres://user:pass@host:5432/swosh \
//	mongo-migrate [-dry-run]
//
// With -dry-run everything is read and inserted inside a transaction that is
// then rolled back, so the counts are exact but no rows are kept. The schema
// migrations still run, as they would on server startup.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sethvargo/go-envconfig"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

type config struct {
	// MongoURI must name the database, as the Kotlin service's URI did.
	MongoURI    string `env:"SWOSH_MONGODB_URI, required"`
	DatabaseURL string `env:"SWOSH_DATABASE_URL, required"`
}

func main() {
	dryRun := flag.Bool("dry-run", false, "import inside a transaction and roll it back")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, log, *dryRun)
	stop()
	if err != nil {
		log.Error("migration failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, dryRun bool) error {
	var cfg config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	swoshes, users, err := readMongo(ctx, cfg.MongoURI)
	if err != nil {
		return err
	}
	log.Info("read from mongodb", "swosh", len(swoshes), "users", len(users))

	if err := db.Migrate(cfg.DatabaseURL, log); err != nil {
		return err
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	st, err := importAll(ctx, pool, swoshes, users, time.Now(), dryRun)
	if err != nil {
		return err
	}
	log.Info("import finished",
		"dry_run", dryRun,
		"swosh_inserted", st.swoshInserted,
		"swosh_existing", st.swoshExisting,
		"swosh_expired", st.swoshExpired,
		"users_inserted", st.usersInserted,
		"users_existing", st.usersExisting,
	)
	return nil
}

func readMongo(ctx context.Context, uri string) (swoshes []mongoSwosh, users []mongoUser, err error) {
	cs, err := connstring.ParseAndValidate(uri)
	if err != nil {
		return nil, nil, errors.New("SWOSH_MONGODB_URI is not a valid MongoDB URI")
	}
	if cs.Database == "" {
		return nil, nil, errors.New("SWOSH_MONGODB_URI must name the database, e.g. mongodb://mongodb/swosh")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to mongodb: %w", err)
	}
	defer func() {
		err = errors.Join(err, client.Disconnect(context.WithoutCancel(ctx)))
	}()

	// Spring Data names collections after the entity class.
	mdb := client.Database(cs.Database)
	if swoshes, err = readAll[mongoSwosh](ctx, mdb.Collection("swosh")); err != nil {
		return nil, nil, err
	}
	if users, err = readAll[mongoUser](ctx, mdb.Collection("user")); err != nil {
		return nil, nil, err
	}
	return swoshes, users, nil
}

func readAll[T any](ctx context.Context, coll *mongo.Collection) ([]T, error) {
	cur, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("read collection %q: %w", coll.Name(), err)
	}
	var docs []T
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode collection %q: %w", coll.Name(), err)
	}
	return docs, nil
}

type stats struct {
	swoshInserted, swoshExisting, swoshExpired int
	usersInserted, usersExisting               int
}

// importAll writes all documents in a single transaction, so a failure leaves
// Postgres as it was. now decides which links count as expired.
func importAll(ctx context.Context, pool *pgxpool.Pool, swoshes []mongoSwosh, users []mongoUser, now time.Time, dryRun bool) (stats, error) {
	var st stats
	tx, err := pool.Begin(ctx)
	if err != nil {
		return st, fmt.Errorf("begin transaction: %w", err)
	}
	// Undoes everything on error or dry run; a no-op after Commit.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := sqlc.New(tx)

	for _, doc := range swoshes {
		params, ok, err := toImportSwosh(doc, now)
		if err != nil {
			return st, err
		}
		if !ok {
			st.swoshExpired++
			continue
		}
		n, err := q.ImportSwosh(ctx, params)
		if err != nil {
			return st, fmt.Errorf("import swosh %q: %w", doc.ID, err)
		}
		if n == 0 {
			st.swoshExisting++
		} else {
			st.swoshInserted++
		}
	}

	for _, doc := range users {
		params, err := toCreateUser(doc)
		if err != nil {
			return st, err
		}
		n, err := q.CreateUser(ctx, params)
		if err != nil {
			return st, fmt.Errorf("import user %q: %w", doc.Username, err)
		}
		if n == 0 {
			st.usersExisting++
		} else {
			st.usersInserted++
		}
	}

	if dryRun {
		return st, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return st, fmt.Errorf("commit: %w", err)
	}
	return st, nil
}
