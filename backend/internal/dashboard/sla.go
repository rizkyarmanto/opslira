package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"opslira/backend/internal/reporting"
)

// sqlNull* wrap database/sql scanners with a DTO-pointer accessor; pgx v5
// routes NULL through the promoted sql.Scanner implementation.
type (
	sqlNullString struct{ sql.NullString }
	sqlNullTime   struct{ sql.NullTime }
	sqlNullInt32  struct{ sql.NullInt32 }
)

func (n sqlNullString) ptr() *string {
	if !n.Valid {
		return nil
	}

	return &n.String
}

func (n sqlNullTime) ptr() *time.Time {
	if !n.Valid {
		return nil
	}

	return &n.Time
}

func (n sqlNullInt32) ptr() *int {
	if !n.Valid {
		return nil
	}
	value := int(n.Int32)

	return &value
}

// cacheTTL matches Laravel's Cache::remember(..., 60) for the NOC SLA
// blocks; they live in Redis DB1.
const cacheTTL = 60 * time.Second

func (h *Handler) cacheGet(ctx context.Context, key string, dest any) bool {
	raw, err := h.cache.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}

	return json.Unmarshal(raw, dest) == nil
}

func (h *Handler) cacheSet(ctx context.Context, key string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = h.cache.Set(ctx, key, raw, cacheTTL).Err()
}

// cachedGlobalSLA is the pooled monthly SLA across active targets, or null
// when no log exists yet (never a fabricated 100%).
func (h *Handler) cachedGlobalSLA(r *http.Request) (*float64, error) {
	const key = "noc:sla:global"
	var cached *float64
	if h.cacheGet(r.Context(), key, &cached) {
		return cached, nil
	}

	loc := h.cfg.Location()
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)

	window, err := h.sla.ForWindow(r.Context(), start, end, nil)
	if err != nil {
		return nil, err
	}

	var out *float64
	if window.SLA != nil {
		rounded := round2(*window.SLA)
		out = &rounded
	}
	h.cacheSet(r.Context(), key, out)

	return out, nil
}

// cachedSLATrend returns the last 30 calendar days (oldest first) with
// uptime, average latency and incident counts.
func (h *Handler) cachedSLATrend(r *http.Request) ([]TrendPoint, error) {
	const key = "noc:sla:trend"
	var cached []TrendPoint
	if h.cacheGet(r.Context(), key, &cached) {
		return cached, nil
	}

	loc := h.cfg.Location()
	today := startOfDay(time.Now().In(loc))
	from := today.AddDate(0, 0, -29)

	incidents := map[string]int{}
	rows, err := h.pool.Query(r.Context(),
		`SELECT to_char(started_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD'), COUNT(*)::int
		   FROM incidents WHERE started_at >= $1
		  GROUP BY 1`, from)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			rows.Close()
			return nil, err
		}
		incidents[day] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	latencies := map[string]float64{}
	rows, err = h.pool.Query(r.Context(),
		`SELECT to_char(checked_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD'),
		        AVG(response_time_ms)::double precision
		   FROM monitoring_logs WHERE checked_at >= $1
		  GROUP BY 1`, from)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var day string
		var avg float64
		if err := rows.Scan(&day, &avg); err != nil {
			rows.Close()
			return nil, err
		}
		latencies[day] = avg
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	points := make([]TrendPoint, 0, 30)
	for i := 29; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		start := day
		end := day.AddDate(0, 0, 1).Add(-time.Nanosecond)

		window, err := h.sla.ForWindow(r.Context(), start, end, nil)
		if err != nil {
			return nil, err
		}

		point := TrendPoint{Date: day.Format("2006-01-02")}
		if window.SLA != nil {
			rounded := round2(*window.SLA)
			point.Uptime = &rounded
		}
		if avg, ok := latencies[point.Date]; ok {
			rounded := math.Round(avg*10) / 10
			point.AvgLatency = &rounded
		}
		point.Incidents = incidents[point.Date]
		points = append(points, point)
	}

	h.cacheSet(r.Context(), key, points)

	return points, nil
}

// cachedTargetDailySLA returns per-target SLA for the latest day with log
// data, worst uptime first; empty when no log exists yet.
func (h *Handler) cachedTargetDailySLA(r *http.Request) ([]TargetDaily, error) {
	const key = "noc:sla:target-daily"
	var cached []TargetDaily
	if h.cacheGet(r.Context(), key, &cached) {
		return cached, nil
	}

	latest, err := h.latestLogDate(r)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		empty := []TargetDaily{}
		h.cacheSet(r.Context(), key, empty)

		return empty, nil
	}

	loc := h.cfg.Location()
	day, err := time.ParseInLocation("2006-01-02", *latest, loc)
	if err != nil {
		return nil, err
	}
	start := day
	end := day.AddDate(0, 0, 1).Add(-time.Nanosecond)

	window, err := h.sla.ForWindow(r.Context(), start, end, nil)
	if err != nil {
		return nil, err
	}
	byTarget := map[string]reporting.TargetWindow{}
	for _, row := range window.Targets {
		byTarget[row.TargetID] = row
	}

	counts := map[string][2]int{}
	rows, err := h.pool.Query(r.Context(),
		`SELECT target_id,
		        COUNT(*)::int,
		        COUNT(*) FILTER (WHERE status = 'DOWN')::int
		   FROM monitoring_logs
		  WHERE checked_at BETWEEN $1 AND $2
		  GROUP BY target_id`, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id    string
			total int
			down  int
		)
		if err := rows.Scan(&id, &total, &down); err != nil {
			rows.Close()
			return nil, err
		}
		counts[id] = [2]int{total, down}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	targets, err := h.activeTargets(r)
	if err != nil {
		return nil, err
	}

	daily := []TargetDaily{}
	for _, target := range targets {
		row, ok := byTarget[target.ID]
		if !ok || row.SLA == nil {
			continue
		}
		uptime := round2(*row.SLA)
		count := counts[target.ID]
		daily = append(daily, TargetDaily{
			TargetID:      target.ID,
			Name:          target.Name,
			CurrentStatus: target.CurrentStatus,
			Uptime:        &uptime,
			TotalChecks:   count[0],
			FailedChecks:  count[1],
		})
	}
	sort.SliceStable(daily, func(i, j int) bool { return *daily[i].Uptime < *daily[j].Uptime })

	h.cacheSet(r.Context(), key, daily)

	return daily, nil
}

// latestLogDate is the newest checked_at as an Asia/Jakarta date, or null
// when no log exists yet.
func (h *Handler) latestLogDate(r *http.Request) (*string, error) {
	var max sql.NullTime
	if err := h.pool.QueryRow(r.Context(), `SELECT MAX(checked_at) FROM monitoring_logs`).Scan(&max); err != nil {
		return nil, err
	}
	if !max.Valid {
		return nil, nil
	}

	date := max.Time.In(h.cfg.Location()).Format("2006-01-02")

	return &date, nil
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
