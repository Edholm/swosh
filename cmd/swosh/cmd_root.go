package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/sethvargo/go-envconfig"
	"github.com/spf13/cobra"

	"github.com/Edholm/swosh/internal/config"
)

// version is set at build time with
// -ldflags "-X main.version=…".
var version = "dev"

// Execute runs the CLI with the process arguments and environment and returns
// the exit code.
func Execute(ctx context.Context) int {
	cmd := NewRootCmd(envconfig.OsLookuper())
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}

// app is the state shared by subcommands. Root's PersistentPreRunE fills it
// in before any subcommand runs.
type app struct {
	cfg *config.Config
	log *slog.Logger
}

// NewRootCmd builds the command tree. Configuration is read from env.
func NewRootCmd(env envconfig.Lookuper) *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:           "swosh",
		Short:         "Short links that open Swish with a prefilled payment",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFrom(cmd.Context(), env)
			if err != nil {
				return err
			}
			a.cfg = cfg
			a.log = slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: cfg.LogLevel}))
			return nil
		},
	}
	root.AddCommand(
		newServeCmd(a),
		newHealthcheckCmd(a),
		newAdminCmd(a),
		newVersionCmd(),
	)
	return root
}
