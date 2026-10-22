package expiry

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

var discard = slog.New(slog.DiscardHandler)

type fakeDeleter struct {
	n     int64
	err   error
	calls chan struct{}
}

func (f *fakeDeleter) DeleteExpiredSwoshes(context.Context, time.Time) (int64, error) {
	if f.calls != nil {
		f.calls <- struct{}{}
	}
	return f.n, f.err
}

func TestSweepCounts(t *testing.T) {
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	Sweep(t.Context(), &fakeDeleter{n: 3}, c, discard)
	Sweep(t.Context(), &fakeDeleter{err: errors.New("db down")}, c, discard)
	if n := testutil.ToFloat64(c); n != 3 {
		t.Errorf("expired_links_total = %v, want 3", n)
	}
}

func TestRunSweepsUntilCancelled(t *testing.T) {
	d := &fakeDeleter{calls: make(chan struct{})}
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		Run(ctx, d, time.Millisecond, c, discard)
		close(done)
	}()

	for range 3 {
		select {
		case <-d.calls:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not sweep")
		}
	}
	cancel()
	// Let a sweep that started before the cancel finish.
	go func() {
		for range d.calls {
		}
	}()
	select {
	case <-done:
		close(d.calls)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestSweepWithPostgres(t *testing.T) {
	url := dbtest.URL(t)
	if err := db.Migrate(url, discard); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Connect(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)

	past, future := time.Now().Add(-time.Minute), time.Now().Add(time.Hour)
	for _, p := range []sqlc.CreateSwoshParams{
		{ID: "past01", Payee: "1230000000", Amount: 1, ExpiresOn: &past},
		{ID: "futur1", Payee: "1230000000", Amount: 1, ExpiresOn: &future},
		{ID: "never1", Payee: "1230000000", Amount: 1},
	} {
		if _, err := q.CreateSwosh(t.Context(), p); err != nil {
			t.Fatal(err)
		}
	}

	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	Sweep(t.Context(), q, c, discard)
	if n := testutil.ToFloat64(c); n != 1 {
		t.Errorf("expired_links_total = %v, want 1", n)
	}
	for _, id := range []string{"futur1", "never1"} {
		if _, err := q.GetSwosh(t.Context(), id); err != nil {
			t.Errorf("GetSwosh(%s) after sweep: %v", id, err)
		}
	}
}
