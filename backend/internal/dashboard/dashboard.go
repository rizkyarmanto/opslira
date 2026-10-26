// Package dashboard serves the NOC overview and the three group dashboards,
// porting DashboardController with its SLA wiring.
package dashboard

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/config"
	"opslira/backend/internal/httpx"
	"opslira/backend/internal/model"
	"opslira/backend/internal/probestate"
	"opslira/backend/internal/reporting"
)

// Handler serves /api/v1/dashboard/*.
type Handler struct {
	pool  *pgxpool.Pool
	rdb   *goredis.Client // DB0: probe state
	cache *goredis.Client // DB1: 60s SLA caches
	sla   *reporting.SLA
	cfg   config.Config
}

// NewHandler wires the dashboard endpoints.
func NewHandler(pool *pgxpool.Pool, rdb, cache *goredis.Client, sla *reporting.SLA, cfg config.Config) *Handler {
	return &Handler{pool: pool, rdb: rdb, cache: cache, sla: sla, cfg: cfg}
}

// Mount registers the dashboard routes behind the session middleware.
func (h *Handler) Mount(r chi.Router, mw func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(mw)
		r.Get("/dashboard/noc", h.noc)
		r.Get("/dashboard/alerts", h.alerts)
		r.Get("/dashboard/network", h.network)
		r.Get("/dashboard/apps-domains-db", h.appsDomainsDb)
		r.Get("/dashboard/mail-s3-beszel", h.mailS3Beszel)
		r.Get("/dashboard/states", h.states)
	})
}

// TrendPoint is one day of the NOC SLA trend chart.
type TrendPoint struct {
	Date       string   `json:"date"`
	Uptime     *float64 `json:"uptime"`
	AvgLatency *float64 `json:"avg_latency"`
	Incidents  int      `json:"incidents"`
}

// TargetDaily is one per-target SLA row for the latest day with log data.
type TargetDaily struct {
	TargetID      string   `json:"target_id"`
	Name          string   `json:"name"`
	CurrentStatus string   `json:"current_status"`
	Uptime        *float64 `json:"uptime"`
	TotalChecks   int      `json:"total_checks"`
	FailedChecks  int      `json:"failed_checks"`
}

// noc renders the executive overview: counters, active targets, open
// incidents and the three cached SLA blocks.
func (h *Handler) noc(w http.ResponseWriter, r *http.Request) {
	targets, err := h.activeTargets(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load dashboard.")
		return
	}

	summary := model.NocSummary{Total: len(targets)}
	for _, target := range targets {
		switch target.CurrentStatus {
		case model.StatusUp:
			summary.Up++
		case model.StatusDegraded:
			summary.Degraded++
		case model.StatusDown:
			summary.Down++
		case model.StatusMaintenance:
			summary.Maintenance++
		}
	}

	incidents, err := h.listIncidents(r, "open", 20)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load incidents.")
		return
	}

	globalSLA, err := h.cachedGlobalSLA(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load SLA.")
		return
	}
	trend, err := h.cachedSLATrend(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load SLA.")
		return
	}
	daily, err := h.cachedTargetDailySLA(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load SLA.")
		return
	}
	slaDate, err := h.latestLogDate(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load SLA.")
		return
	}

	summary.GlobalSLA = globalSLA
	httpx.JSON(w, http.StatusOK, map[string]any{
		"summary":          summary,
		"targets":          targets,
		"incidents":        incidents,
		"global_sla":       globalSLA,
		"sla_window_days":  daysInCurrentMonth(h.cfg.Location()),
		"sla_trend":        trend,
		"target_daily_sla": daily,
		"sla_date":         slaDate,
	})
}

// alerts lists incidents filtered open|resolved|all (default open).
func (h *Handler) alerts(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	switch filter {
	case "open", "resolved", "all":
	default:
		filter = "open"
	}

	incidents, err := h.listIncidents(r, filter, 100)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load incidents.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"incidents": incidents,
		"filter":    filter,
	})
}

// network renders dashboard 1: NETWORK devices with state, heartbeat and
// the SNMP traffic summary, plus the INTERNET subset.
func (h *Handler) network(w http.ResponseWriter, r *http.Request) {
	targets, err := h.activeTargets(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load dashboard.")
		return
	}
	networkTargets := filterByGroup(targets, "NETWORK")

	internet := []model.Target{}
	for _, target := range networkTargets {
		if target.TargetType == "INTERNET" {
			internet = append(internet, target)
		}
	}

	decorated := h.decorate(r, networkTargets, true)
	decoratedInternet := h.decorate(r, internet, true)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"targets":          decorated,
		"internet_targets": decoratedInternet,
		"traffic_summary":  trafficSummary(decorated),
	})
}

// appsDomainsDb renders dashboard 2 with environment tab counts.
func (h *Handler) appsDomainsDb(w http.ResponseWriter, r *http.Request) {
	environment := r.URL.Query().Get("environment")

	targets, err := h.activeTargets(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load dashboard.")
		return
	}
	base := filterByGroup(targets, "APP_DOMAIN_DB")

	counts := map[string]int{"ALL": len(base), "PRODUCTION": 0, "STAGING": 0, "DEVELOPMENT": 0}
	for _, target := range base {
		if _, tracked := counts[target.Environment]; tracked && target.Environment != "ALL" {
			counts[target.Environment]++
		}
	}

	scoped := base
	if environment != "" && environment != "ALL" {
		scoped = filterByEnvironment(base, environment)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"targets":     h.decorate(r, scoped, true),
		"environment": environment,
		"counts":      counts,
	})
}

// mailS3Beszel renders dashboard 3.
func (h *Handler) mailS3Beszel(w http.ResponseWriter, r *http.Request) {
	targets, err := h.activeTargets(r)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load dashboard.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"targets": h.decorate(r, filterByGroup(targets, "MAIL_S3_BESZEL"), true),
	})
}

// states is the lightweight polling endpoint: id -> current status.
func (h *Handler) states(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, current_status, last_checked_at FROM monitoring_targets WHERE is_active = true`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load states.")
		return
	}
	defer rows.Close()

	type stateRow struct {
		CurrentStatus string     `json:"current_status"`
		LastCheckedAt *time.Time `json:"last_checked_at"`
	}
	statuses := map[string]stateRow{}
	for rows.Next() {
		var (
			id          string
			status      string
			lastChecked sqlNullTime
		)
		if err := rows.Scan(&id, &status, &lastChecked); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load states.")
			return
		}
		statuses[id] = stateRow{CurrentStatus: status, LastCheckedAt: lastChecked.ptr()}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"statuses": statuses})
}

func (h *Handler) activeTargets(r *http.Request) ([]model.Target, error) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+model.Columns+` FROM monitoring_targets WHERE is_active = true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := []model.Target{}
	for rows.Next() {
		target, err := model.ScanTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	return targets, rows.Err()
}

// listIncidents returns incidents with the target name joined, newest first.
func (h *Handler) listIncidents(r *http.Request, filter string, limit int) ([]model.Incident, error) {
	where := ""
	switch filter {
	case "open":
		where = "WHERE i.status <> 'RESOLVED'"
	case "resolved":
		where = "WHERE i.status = 'RESOLVED'"
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT i.id, i.target_id, t.name, i.severity, i.status, i.root_cause_message,
		        i.started_at, i.resolved_at, i.duration_seconds
		   FROM incidents i
		   LEFT JOIN monitoring_targets t ON t.id = i.target_id
		   `+where+`
		   ORDER BY i.started_at DESC
		   LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	incidents := []model.Incident{}
	for rows.Next() {
		var (
			incident model.Incident
			name     sqlNullString
			resolved sqlNullTime
			duration sqlNullInt32
		)
		if err := rows.Scan(&incident.ID, &incident.TargetID, &name, &incident.Severity,
			&incident.Status, &incident.RootCauseMessage, &incident.StartedAt, &resolved,
			&duration); err != nil {
			return nil, err
		}
		incident.TargetName = name.ptr()
		incident.ResolvedAt = resolved.ptr()
		incident.DurationSeconds = duration.ptr()
		incidents = append(incidents, incident)
	}

	return incidents, rows.Err()
}

// decorate adds the Redis state and the last-60 heartbeat to each target
// (withState paths of DashboardController::decorateTargets).
func (h *Handler) decorate(r *http.Request, targets []model.Target, withState bool) []model.Target {
	if !withState || len(targets) == 0 {
		return targets
	}

	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	heartbeats := h.heartbeats(r, ids)

	decorated := make([]model.Target, 0, len(targets))
	for _, target := range targets {
		state, err := probestate.Get(r.Context(), h.rdb, target.ID)
		if err != nil {
			state = model.TargetState{Status: model.StatusPending, SummaryMetrics: map[string]any{}}
		}
		target.State = &state
		target.Heartbeat = heartbeats[target.ID]
		decorated = append(decorated, target)
	}

	return decorated
}

// heartbeats loads each target's last 60 check outcomes, oldest first.
func (h *Handler) heartbeats(r *http.Request, ids []string) map[string][]string {
	rows, err := h.pool.Query(r.Context(),
		`SELECT target_id, status FROM (
		    SELECT target_id, status,
		           row_number() OVER (PARTITION BY target_id ORDER BY checked_at DESC) AS rank
		      FROM monitoring_logs
		     WHERE target_id = ANY($1)
		 ) ranked
		 WHERE rank <= 60
		 ORDER BY target_id, rank DESC`, ids)
	if err != nil {
		return map[string][]string{}
	}
	defer rows.Close()

	// Rows arrive newest-first per target; collect then reverse so the
	// heartbeat reads oldest first.
	collected := map[string][]string{}
	order := []string{}
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return map[string][]string{}
		}
		if _, seen := collected[id]; !seen {
			order = append(order, id)
		}
		collected[id] = append(collected[id], status)
	}

	out := make(map[string][]string, len(collected))
	for _, id := range order {
		statuses := collected[id]
		for i, j := 0, len(statuses)-1; i < j; i, j = i+1, j-1 {
			statuses[i], statuses[j] = statuses[j], statuses[i]
		}
		out[id] = statuses
	}

	return out
}

func filterByGroup(targets []model.Target, group string) []model.Target {
	out := []model.Target{}
	for _, target := range targets {
		if target.DashboardGroup == group {
			out = append(out, target)
		}
	}

	return out
}

func filterByEnvironment(targets []model.Target, environment string) []model.Target {
	out := []model.Target{}
	for _, target := range targets {
		if strings.EqualFold(target.Environment, environment) {
			out = append(out, target)
		}
	}

	return out
}

// trafficSummary maps target name -> SNMP rx/tx Mbps for the area chart.
func trafficSummary(targets []model.Target) map[string]map[string]float64 {
	summary := map[string]map[string]float64{}
	for _, target := range targets {
		rx, tx := 0.0, 0.0
		if target.State != nil && target.State.SummaryMetrics != nil {
			rx = asFloat(target.State.SummaryMetrics["rx_mbps"])
			tx = asFloat(target.State.SummaryMetrics["tx_mbps"])
		}
		summary[target.Name] = map[string]float64{"rx_mbps": rx, "tx_mbps": tx}
	}

	return summary
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint64:
		return float64(typed)
	default:
		return 0
	}
}

func daysInCurrentMonth(loc *time.Location) int {
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, loc)

	return int(next.Sub(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)).Hours() / 24)
}
