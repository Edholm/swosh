// Package expiry deletes links whose expiry time has passed.
package expiry

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// initialDelay is how long Run waits before the first sweep, like the
// Kotlin scheduler did.
const initialDelay = 1337 * time.Millisecond

// Deleter removes all links that expired before now and returns how many.
type Deleter interface {
	DeleteExpiredSwoshes(ctx context.Context, now time.Time) (int64, error)
}

// Run deletes expired links every interval until ctx is cancelled. Failed
// sweeps are logged and retried on the next tick.
func Run(ctx context.Context, db Deleter, interval time.Duration, expired prometheus.Counter, log *slog.Logger) {
	timer := time.NewTimer(initialDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		Sweep(ctx, db, expired, log)
		timer.Reset(interval)
	}
}

// Sweep deletes the links that have expired by now.
func Sweep(ctx context.Context, db Deleter, expired prometheus.Counter, log *slog.Logger) {
	now := time.Now()
	n, err := db.DeleteExpiredSwoshes(ctx, now)
	if err != nil {
		if ctx.Err() == nil {
			log.ErrorContext(ctx, "delete expired links", "err", err)
		}
		return
	}
	if n > 0 {
		log.InfoContext(ctx, "deleted expired links", "count", n, "before", now)
	}
	expired.Add(float64(n))
}
