// Package runner owns every periodic job in Opslira: claiming due targets,
// running probes through the PHP adapter endpoints, dispatching reports, and
// firing the two daily maintenance tasks.
//
// Probes themselves stay in PHP because each adapter depends on PHP-only
// libraries; this process only decides what runs, how much runs at once, and
// when.
package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"opslira/runner/internal/client"
	"opslira/runner/internal/config"
)

// shutdownGrace is how long in-flight probes get to finish after a signal.
const shutdownGrace = 30 * time.Second

type Runner struct {
	cfg    config.Config
	client *client.Client

	inflight  atomic.Int64
	mu        sync.Mutex
	lastClaim time.Time
}

func New(cfg config.Config, c *client.Client) *Runner {
	return &Runner{cfg: cfg, client: c}
}

// Run blocks until ctx is cancelled, then shuts down cleanly: claiming stops,
// the target channel closes, and workers get shutdownGrace to finish.
func (r *Runner) Run(ctx context.Context) error {
	loc, err := time.LoadLocation(r.cfg.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone %s: %w", r.cfg.Timezone, err)
	}

	ids := make(chan string)

	var workers sync.WaitGroup
	for range r.cfg.ProbeConcurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r.worker(ctx, ids)
		}()
	}

	var loops sync.WaitGroup
	loops.Add(1)
	go func() {
		defer loops.Done()
		r.claimLoop(ctx, ids)
	}()
	loops.Add(1)
	go func() {
		defer loops.Done()
		r.reportLoop(ctx)
	}()
	for _, task := range []struct{ name, at string }{
		{name: "rollup", at: r.cfg.RollupAt},
		{name: "prune", at: r.cfg.PruneAt},
	} {
		loops.Add(1)
		go func(task string, at string) {
			defer loops.Done()
			r.dailyLoop(ctx, loc, task, at)
		}(task.name, task.at)
	}

	srv := &http.Server{Addr: r.cfg.ListenAddr, Handler: r.healthHandler()}
	var server sync.WaitGroup
	server.Add(1)
	go func() {
		defer server.Done()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logEvent("error", "health server failed", "addr", r.cfg.ListenAddr, "err", err)
		}
	}()

	logEvent("info", "runner started",
		"concurrency", r.cfg.ProbeConcurrency,
		"tick_s", r.cfg.TickSeconds,
		"report_tick_s", r.cfg.ReportTickSeconds,
		"listen", r.cfg.ListenAddr,
	)

	<-ctx.Done()
	logEvent("info", "shutdown requested")

	// Claiming stops with ctx; this waits for the probes already in flight.
	done := make(chan struct{})
	go func() {
		loops.Wait()
		workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		logEvent("info", "shutdown complete")
	case <-time.After(shutdownGrace):
		logEvent("error", "shutdown grace elapsed with probes still running")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logEvent("error", "health server shutdown failed", "err", err)
	}
	server.Wait()

	return nil
}

func (r *Runner) worker(ctx context.Context, ids <-chan string) {
	// Probes run on a context detached from the signal: the shutdown grace only
	// means something if an in-flight request is allowed to finish instead of
	// being cancelled the moment SIGINT arrives.
	probeCtx := context.WithoutCancel(ctx)

	for id := range ids {
		start := time.Now()
		r.inflight.Add(1)
		outcome, err := r.client.RunProbe(probeCtx, id)
		r.inflight.Add(-1)

		if err != nil {
			logEvent("error", "probe failed", "id", id, "err", err)
			continue
		}
		logEvent("info", "probe done",
			"id", id,
			"status", outcome.Status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// claimLoop is the only sender on ids, so it also closes the channel: once it
// returns, every worker drains what it holds and exits.
func (r *Runner) claimLoop(ctx context.Context, ids chan<- string) {
	defer close(ids)

	ticker := time.NewTicker(time.Duration(r.cfg.TickSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if int(r.inflight.Load()) >= r.cfg.ProbeConcurrency {
				continue
			}

			targets, err := r.client.Claim(ctx, r.cfg.ProbeConcurrency*2)
			if err != nil {
				logEvent("error", "claim failed", "err", err)
				continue
			}
			r.setLastClaim(time.Now())
			logEvent("info", "claimed targets", "targets", len(targets))

			for _, target := range targets {
				select {
				case ids <- target.ID:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (r *Runner) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(r.cfg.ReportTickSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			dispatched, err := r.client.DispatchReports(ctx)
			if err != nil {
				logEvent("error", "report dispatch failed", "err", err)
				continue
			}
			if dispatched > 0 {
				logEvent("info", "reports dispatched",
					"dispatched", dispatched,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
		}
	}
}

// dailyLoop fires one maintenance task at hhmm in loc, then re-arms for the
// next day.
func (r *Runner) dailyLoop(ctx context.Context, loc *time.Location, task, hhmm string) {
	timer := time.NewTimer(time.Until(nextDaily(time.Now(), hhmm, loc)))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			start := time.Now()
			if _, err := r.client.RunMaintenance(ctx, task); err != nil {
				logEvent("error", "maintenance failed", "task", task, "err", err)
			} else {
				logEvent("info", "maintenance done",
					"task", task,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
			timer.Reset(time.Until(nextDaily(time.Now(), hhmm, loc)))
		}
	}
}

// nextDaily returns the next hhmm occurrence strictly after now, in loc.
func nextDaily(now time.Time, hhmm string, loc *time.Location) time.Time {
	local := now.In(loc)
	hour := int(hhmm[0]-'0')*10 + int(hhmm[1]-'0')
	minute := int(hhmm[3]-'0')*10 + int(hhmm[4]-'0')

	candidate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}

	return candidate
}

func (r *Runner) healthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		last := r.lastClaimTime()
		lastClaim := ""
		if !last.IsZero() {
			lastClaim = last.UTC().Format(time.RFC3339)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"inflight":%d,"last_claim":%q}`, r.inflight.Load(), lastClaim)
	})

	return mux
}

func (r *Runner) setLastClaim(at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastClaim = at
}

func (r *Runner) lastClaimTime() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lastClaim
}

// logEvent writes one logfmt line: ts=... level=info msg="..." key=value ...
func logEvent(level, msg string, kv ...any) {
	var line strings.Builder
	fmt.Fprintf(&line, "ts=%s level=%s msg=%q", time.Now().Format(time.RFC3339), level, msg)
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&line, " %v=%v", kv[i], kv[i+1])
	}

	fmt.Fprintln(os.Stdout, line.String())
}
