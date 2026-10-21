// Opslira runner: one process that owns all periodic work.
//
// It claims due targets from Laravel over the internal token-authenticated
// endpoints, runs them through a bounded worker pool, dispatches due reports,
// and fires the daily rollup and partition prune. Probe execution stays in PHP;
// this binary only schedules it.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"opslira/runner/internal/client"
	"opslira/runner/internal/config"
	"opslira/runner/internal/runner"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runner.New(cfg, client.New(cfg)).Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "runner error:", err)
		os.Exit(1)
	}
}
