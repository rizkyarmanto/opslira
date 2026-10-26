package reporting

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/config"
	"opslira/backend/internal/mailer"
)

// Handler serves the /reports endpoints and drives the report scheduler tick.
type Handler struct {
	pool      *pgxpool.Pool
	generator *Generator
	sla       *SLA
	cfg       config.Config
	logger    *slog.Logger
}

// NewHandler wires report schedules, generation and SLA services.
func NewHandler(pool *pgxpool.Pool, m *mailer.Mailer, cfg config.Config, logger *slog.Logger) *Handler {
	sla := NewSLA(pool)
	generator := NewGenerator(pool, sla, m, cfg, logger)

	return &Handler{pool: pool, generator: generator, sla: sla, cfg: cfg, logger: logger}
}

// SLA exposes the shared SLA service for the dashboard package.
func (h *Handler) SLA() *SLA { return h.sla }

// Mount registers the /reports routes behind the session middleware.
func (h *Handler) Mount(r chi.Router, mw func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(mw)
		r.Get("/reports/schedules", h.schedules)
		r.Post("/reports/schedules", h.store)
		r.Put("/reports/schedules/{scheduleID}", h.update)
		r.Delete("/reports/schedules/{scheduleID}", h.destroy)
		r.Post("/reports/schedules/{scheduleID}/run-now", h.runNow)
		r.Get("/reports/email-logs", h.emailLogs)
		r.Get("/reports/download/{reportID}/{format}", h.download)
	})
}

// DispatchDue runs every schedule whose next_run_at has passed and advances
// the cron window on success (ReportDispatchService::dispatchDue). A failure
// propagates so the schedule stays due and the next tick retries it.
func (h *Handler) DispatchDue(ctx context.Context) (int, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT `+scheduleColumns+` FROM report_schedules
		  WHERE is_active = true AND next_run_at <= now()
		  ORDER BY next_run_at`)
	if err != nil {
		return 0, err
	}

	due := []Schedule{}
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, schedule)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	dispatched := 0
	for _, schedule := range due {
		if _, err := h.generator.Run(ctx, schedule); err != nil {
			return dispatched, err
		}

		nextRun, err := NextCronRun(schedule.CronExpression, schedule.Timezone, time.Now())
		if err != nil {
			return dispatched, err
		}
		if _, err := h.pool.Exec(ctx,
			`UPDATE report_schedules SET last_run_at = now(), next_run_at = $2, updated_at = now() WHERE id = $1`,
			schedule.ID, nextRun); err != nil {
			return dispatched, err
		}
		dispatched++
	}

	return dispatched, nil
}

// storagePath resolves a stored relative artifact path inside the report
// storage root, rejecting traversal attempts.
func (h *Handler) storagePath(relative string) (string, bool) {
	normalized := strings.ReplaceAll(relative, "\\", "/")
	if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, "..") || strings.Contains(normalized, ":") {
		return "", false
	}

	return strings.TrimSuffix(h.cfg.ReportStorageDir, "/") + "/" + normalized, true
}
