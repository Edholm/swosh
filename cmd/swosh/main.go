// Command swosh runs and manages the swosh service. See `swosh --help`.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := Execute(ctx)
	stop()
	os.Exit(code)
}
