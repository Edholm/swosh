package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/Edholm/swosh/internal/db"
)

const shutdownTimeout = 10 * time.Second

func newServeCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the swosh server",
		Long:  "Run the swosh server. Pending database migrations are applied on startup.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.serve(cmd.Context())
		},
	}
}

func (a *app) serve(ctx context.Context) error {
	a.log.Info("starting swosh", "version", version)

	if err := db.Migrate(a.cfg.DatabaseURL, a.log); err != nil {
		return err
	}
	pool, err := db.Connect(ctx, a.cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.Handle("GET /health", healthHandler(pool, a.log))
	actuator := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(a.cfg.ActuatorPort)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return runServer(ctx, a.log, actuator)
}

// runServer serves srv until ctx is cancelled, then shuts it down gracefully.
func runServer(ctx context.Context, log *slog.Logger, srv *http.Server) error {
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	log.Info("listening", "addr", lis.Addr().String())

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(lis) }()

	select {
	case err := <-errc:
		return fmt.Errorf("serve %s: %w", srv.Addr, err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down %s: %w", srv.Addr, err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve %s: %w", srv.Addr, err)
	}
	return nil
}

type pinger interface {
	Ping(ctx context.Context) error
}

// healthHandler reports UP while the database answers and DOWN with a 503
// otherwise, in the same shape as Spring Boot's actuator.
func healthHandler(db pinger, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, code := "UP", http.StatusOK
		if err := db.Ping(ctx); err != nil {
			log.WarnContext(ctx, "health check failed", "err", err)
			status, code = "DOWN", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})
}
