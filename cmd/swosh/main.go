// Command swosh runs and manages the swosh service. See `swosh --help`.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Edholm/swosh/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx)
	stop()
	os.Exit(code)
}
