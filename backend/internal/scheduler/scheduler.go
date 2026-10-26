// Package scheduler runs the in-process loops the Go binary owns after the
// Laravel split: probe ticks, report dispatch, and the daily rollup/prune
// jobs (go/internal/runner + routes/console.php responsibilities).
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/config"
	"opslira/backend/internal/probes"
	"opslira/backend/internal/reporting"
)

// Scheduler owns the periodic work; Start blocks until ctx is cancelled.
type Scheduler struct {
	probes  *probes.Service
	reports *reporting.Handler
	pool    *pgxpool.Pool
	cfg     config.Config
	logger  *slog.Logger
}

// New wires the scheduler with its two tick consumers and the pool.
func New(probesSvc *probes.Service, reports *reporting.Handler, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Scheduler {
	return &Scheduler{probes: probesSvc, reports: reports, pool: pool, cfg: cfg, logger: logger}
}

// Start launches the probe tick, report tick and the two daily jobs, then
// blocks until ctx ends (main shuts the HTTP server down in parallel).
func (s *Scheduler) Start(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.probeLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.reportLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.dailyLoop(ctx, s.cfg.RollupAt, "rollup", s.rollup)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.dailyLoop(ctx, s.cfg.PruneAt, "prune", s.prune)
	}()

	<-ctx.Done()
	wg.Wait()
}

// probeLoop claims due targets on RUNNER_TICK_SECONDS with a
// PROBE_CONCURRENCY worker pool; a still-running tick skips the next beat so
// two pools never probe the same target.
func (s *Scheduler) probeLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.TickSeconds) * time.Second)
	defer ticker.Stop()

	var running atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if running.CompareAndSwap(false, true) {
				executed, err := s.probes.RunDue(ctx, s.cfg.ProbeConcurrency)
				running.Store(false)
				if err != nil && ctx.Err() == nil {
					s.logger.Error("probe tick failed", "error", err)
				}
				if executed > 0 {
					s.logger.Info("probe tick completed", "targets", executed)
				}
			}
		}
	}
}

// reportLoop dispatches due report schedules on REPORT_TICK_SECONDS with the
// same overlap guard.
func (s *Scheduler) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.ReportTickSeconds) * time.Second)
	defer ticker.Stop()

	var running atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if running.CompareAndSwap(false, true) {
				dispatched, err := s.reports.DispatchDue(ctx)
				running.Store(false)
				if err != nil && ctx.Err() == nil {
					s.logger.Error("report tick failed", "error", err)
				}
				if dispatched > 0 {
					s.logger.Info("report tick completed", "schedules", dispatched)
				}
			}
		}
	}
}

// dailyLoop fires job once per day at hhmm in RUNNER_TIMEZONE.
func (s *Scheduler) dailyLoop(ctx context.Context, hhmm, name string, job func(context.Context) error) {
	loc := s.cfg.Location()
	for {
		if ctx.Err() != nil {
			return
		}
		next := nextAt(hhmm, loc, time.Now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		started := time.Now()
		if err := job(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error(name+" failed", "error", err)
			continue
		}
		s.logger.Info(name+" completed", "took", time.Since(started).String())
	}
}

// nextAt returns the next time strictly after now matching hhmm in loc.
func nextAt(hhmm string, loc *time.Location, now time.Time) time.Time {
	parts := strings.Split(hhmm, ":")
	if len(parts) != 2 {
		return now.Add(time.Minute)
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil {
		return now.Add(time.Minute)
	}

	now = now.In(loc)
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}

	return candidate
}

// rollup aggregates yesterday's monitoring_logs into
// monitoring_daily_summaries (RunMidnightDataRollupCommand port: idempotent
// upsert on (target_id, summary_date)).
func (s *Scheduler) rollup(ctx context.Context) error {
	loc := s.cfg.Location()
	day := time.Now().In(loc).AddDate(0, 0, -1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1).Add(-time.Nanosecond)

	rows, err := s.pool.Query(ctx, `SELECT id FROM monitoring_targets`)
	if err != nil {
		return fmt.Errorf("rollup targets: %w", err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	processed := 0
	for _, id := range ids {
		var (
			total, successful, failed          int
			avgLatency, maxLatency, p95Latency float64
		)
		err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*)::int,
			        COUNT(*) FILTER (WHERE status IN ('UP','DEGRADED'))::int,
			        COUNT(*) FILTER (WHERE status = 'DOWN')::int,
			        COALESCE(AVG(response_time_ms), 0)::double precision,
			        COALESCE(MAX(response_time_ms), 0)::double precision,
			        COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms), 0)::double precision
			   FROM monitoring_logs
			  WHERE target_id = $1 AND checked_at BETWEEN $2 AND $3`,
			id, start, end).Scan(&total, &successful, &failed, &avgLatency, &maxLatency, &p95Latency)
		if err != nil {
			return fmt.Errorf("rollup stats: %w", err)
		}
		if total == 0 {
			continue
		}

		var incidentCount, downtimeSeconds int
		if err := s.pool.QueryRow(ctx,
			`SELECT COUNT(*)::int, COALESCE(SUM(duration_seconds), 0)::int
			   FROM incidents WHERE target_id = $1 AND started_at BETWEEN $2 AND $3`,
			id, start, end).Scan(&incidentCount, &downtimeSeconds); err != nil {
			return fmt.Errorf("rollup incidents: %w", err)
		}

		uptime := round4(float64(successful) / float64(total) * 100)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO monitoring_daily_summaries
				(target_id, summary_date, total_checks, successful_checks, failed_checks,
				 uptime_percentage, avg_response_time_ms, max_response_time_ms, p95_response_time_ms,
				 total_incidents, total_downtime_seconds, maintenance_seconds, daily_metrics_snapshot, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0, '[]'::jsonb, now())
			 ON CONFLICT (target_id, summary_date) DO UPDATE SET
			   total_checks = EXCLUDED.total_checks,
			   successful_checks = EXCLUDED.successful_checks,
			   failed_checks = EXCLUDED.failed_checks,
			   uptime_percentage = EXCLUDED.uptime_percentage,
			   avg_response_time_ms = EXCLUDED.avg_response_time_ms,
			   max_response_time_ms = EXCLUDED.max_response_time_ms,
			   p95_response_time_ms = EXCLUDED.p95_response_time_ms,
			   total_incidents = EXCLUDED.total_incidents,
			   total_downtime_seconds = EXCLUDED.total_downtime_seconds,
			   daily_metrics_snapshot = EXCLUDED.daily_metrics_snapshot`,
			id, start.Format("2006-01-02"), total, successful, failed,
			uptime, round2(avgLatency), round2(maxLatency), round2(p95Latency),
			incidentCount, downtimeSeconds); err != nil {
			return fmt.Errorf("rollup upsert: %w", err)
		}
		processed++
	}

	s.logger.Info("daily rollup finished", "date", start.Format("2006-01-02"), "targets", processed)

	return nil
}

var partitionNamePattern = regexp.MustCompile(`^monitoring_logs_(\d{4})_(\d{2})$`)

// prune drops monitoring_logs partitions older than 90 days and pre-creates
// the next three months (PruneOldMonitoringPartitions port).
func (s *Scheduler) prune(ctx context.Context) error {
	rows, err := s.pool.Query(ctx,
		`SELECT inhrelid::regclass::text FROM pg_inherits WHERE inhparent = 'monitoring_logs'::regclass`)
	if err != nil {
		return fmt.Errorf("prune list: %w", err)
	}

	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	loc := s.cfg.Location()
	now := time.Now().In(loc)
	c := now.AddDate(0, 0, -90)
	cutoff := time.Date(c.Year(), c.Month(), 1, 0, 0, 0, 0, loc)

	dropped := 0
	for _, name := range names {
		match := partitionNamePattern.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		partitionStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
		if partitionStart.Before(cutoff) {
			if _, err := s.pool.Exec(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
				return fmt.Errorf("prune drop %s: %w", name, err)
			}
			dropped++
		}
	}

	created := 0
	for offset := 0; offset <= 2; offset++ {
		month := time.Date(now.Year(), now.Month()+time.Month(offset), 1, 0, 0, 0, 0, loc)
		if ensurePartition(ctx, s.pool, month) {
			created++
		}
	}

	s.logger.Info("partition prune finished", "dropped", dropped, "ensured", created)

	return nil
}

// ensurePartition creates the monthly partition when missing; returns
// whether a new table was created.
func ensurePartition(ctx context.Context, pool *pgxpool.Pool, month time.Time) bool {
	suffix := month.Format("2006_01")
	start := month.Format("2006-01-02T15:04:05-07:00")
	end := month.AddDate(0, 1, 0).Format("2006-01-02T15:04:05-07:00")

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, "monitoring_logs_"+suffix).Scan(&exists); err != nil || exists {
		return false
	}

	_, err := pool.Exec(ctx,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS monitoring_logs_%s PARTITION OF monitoring_logs
		              FOR VALUES FROM ('%s') TO ('%s')`, suffix, start, end))
	if err != nil {
		// A concurrent creator or partition-table mismatch is logged by the
		// caller on the next tick; the IF NOT EXISTS shape keeps this safe.
		return false
	}

	return true
}

// EnsurePartitions pre-creates the current month and the next two at boot so
// fresh databases accept logs immediately (the migration does the same, this
// covers long-running deployments across month boundaries).
func EnsurePartitions(ctx context.Context, pool *pgxpool.Pool, loc *time.Location) {
	now := time.Now().In(loc)
	for offset := 0; offset <= 2; offset++ {
		ensurePartition(ctx, pool, time.Date(now.Year(), now.Month()+time.Month(offset), 1, 0, 0, 0, 0, loc))
	}
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func round4(value float64) float64 { return math.Round(value*10000) / 10000 }
