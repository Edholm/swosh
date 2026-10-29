package expiry

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/dbtest"
	"github.com/Edholm/swosh/internal/db/sqlc"
	"github.com/Edholm/swosh/internal/logtest"
)

type fakeDeleter struct {
	ids   []string
	err   error
	calls chan struct{}
}

func (f *fakeDeleter) DeleteExpiredSwoshes(context.Context, time.Time) ([]string, error) {
	if f.calls != nil {
		f.calls <- struct{}{}
	}
	return f.ids, f.err
}

func TestSweepCounts(t *testing.T) {
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	Sweep(t.Context(), &fakeDeleter{ids: []string{"a", "b", "c"}}, c)
	Sweep(t.Context(), &fakeDeleter{err: errors.New("db down")}, c)
	if n := testutil.ToFloat64(c); n != 3 {
		t.Errorf("expired_links_total = %v, want 3", n)
	}
}

func TestSweepLogsIDs(t *testing.T) {
	buf := logtest.Capture(t)
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	Sweep(t.Context(), &fakeDeleter{ids: []string{"brave-otter-sings", "abc123"}}, c)
	var got struct {
		Msg   string
		Count int
		IDs   []string
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode log %q: %v", buf.String(), err)
	}
	if got.Msg != "deleted expired links" || got.Count != 2 || !slices.Equal(got.IDs, []string{"brave-otter-sings", "abc123"}) {
		t.Errorf("log = %s", buf.String())
	}
}

func TestRunSweepsUntilCancelled(t *testing.T) {
	d := &fakeDeleter{calls: make(chan struct{})}
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "expired_links_total"})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		Run(ctx, d, time.Millisecond, c)
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
	if err := db.Migrate(url); err != nil {
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
	Sweep(t.Context(), q, c)
	if n := testutil.ToFloat64(c); n != 1 {
		t.Errorf("expired_links_total = %v, want 1", n)
	}
	for _, id := range []string{"futur1", "never1"} {
		if _, err := q.GetSwosh(t.Context(), id); err != nil {
			t.Errorf("GetSwosh(%s) after sweep: %v", id, err)
		}
	}
}
