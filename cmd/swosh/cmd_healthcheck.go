package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const healthcheckTimeout = 3 * time.Second

func newHealthcheckCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "healthcheck",
		Short: "Check that the local swosh server is healthy",
		Long: "Request /health on the actuator port of the server running on this machine and " +
			"exit non-zero unless it is UP. The image has no shell or curl, so container health " +
			"checks run this.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(a.cfg.ActuatorPort))
			return checkHealth(cmd.Context(), "http://"+addr+"/health")
		},
	}
}

func checkHealth(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s returned %s", url, resp.Status)
	}
	return nil
}
