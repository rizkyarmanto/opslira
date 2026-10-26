package reporting

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/model"
)

// Segment is a closed time interval used for maintenance and DOWN windows.
type Segment struct {
	Start time.Time
	End   time.Time
}

// WindowCore is the pure result of DowntimeForWindow (SlaPeriodService
// math): whole seconds plus a full-precision SLA percentage.
type WindowCore struct {
	TotalSeconds       int
	MaintenanceSeconds int
	DowntimeSeconds    int
	SLA                *float64
	HasData            bool
}

// TargetWindow is one target's row inside a pooled dashboard window.
type TargetWindow struct {
	TargetID           string
	SLA                *float64
	HasData            bool
	DowntimeSeconds    int
	MaintenanceSeconds int
	TotalSeconds       int
}

// WindowResult is the pooled + per-target outcome of ForWindow.
type WindowResult struct {
	SLA                *float64
	HasData            bool
	DowntimeSeconds    int
	MaintenanceSeconds int
	TotalSeconds       int
	Targets            []TargetWindow
}

// TargetSLA is the check-count report row (SlaCalculatorService).
type TargetSLA struct {
	UptimePercentage     float64 `json:"uptime_percentage"`
	MTTRSeconds          float64 `json:"mttr_seconds"`
	MTBFSeconds          float64 `json:"mtbf_seconds"`
	TotalDowntimeSeconds int     `json:"total_downtime_seconds"`
	IncidentCount        int     `json:"incident_count"`
	TotalChecks          int     `json:"total_checks"`
}

// SLA provides the DB-backed SLA queries; the window math itself is pure.
type SLA struct {
	pool *pgxpool.Pool
}

// NewSLA wires the SLA services to the pool.
func NewSLA(pool *pgxpool.Pool) *SLA { return &SLA{pool: pool} }

// DowntimeForWindow is SlaPeriodService::downtimeForWindow ported 1:1:
// clip both segment lists to the window, merge overlaps, subtract the
// maintenance/DOWN intersection (maintenance wins), and derive the percentage
// from the returned integers. hasData=false forces a nil SLA.
func DowntimeForWindow(start, end time.Time, maintenance, down []Segment, hasData bool) WindowCore {
	core := WindowCore{HasData: hasData}

	// abs first so a reversed window never yields a negative total, round
	// because end-of-day carries .999999 microseconds.
	total := int(math.Round(math.Abs(end.Sub(start).Seconds())))
	core.TotalSeconds = total

	mergedMaintenance := mergeSegments(clipToWindow(maintenance, start, end))
	mergedDown := mergeSegments(clipToWindow(down, start, end))

	core.MaintenanceSeconds = int(math.Round(sumSegmentSeconds(mergedMaintenance)))
	rawDown := sumSegmentSeconds(mergedDown)
	intersection := intersectionSeconds(mergedMaintenance, mergedDown)
	core.DowntimeSeconds = int(math.Round(math.Max(0, rawDown-intersection)))

	denominator := total - core.MaintenanceSeconds
	if hasData && denominator > 0 {
		sla := (float64(denominator-core.DowntimeSeconds) / float64(denominator)) * 100
		core.SLA = &sla
	}

	return core
}

// Daily is one calendar day in Asia/Jakarta, rounded to 4 decimals at the
// return edge (SlaPeriodService::daily).
func Daily(target model.Target, date time.Time, loc *time.Location, maintenance, down []Segment, hasData bool) (dateStr string, sla *float64, core WindowCore) {
	day := date.In(loc)
	start := startOfDay(day)
	end := endOfDay(day)

	core = DowntimeForWindow(start, end, maintenance, down, hasData)

	return start.Format("2006-01-02"), round4Ptr(core.SLA), core
}

// Monthly is the calendar month containing anyDateInMonth, aggregated in one
// core call (SlaPeriodService::monthly) — never an average of daily values.
func Monthly(target model.Target, anyDateInMonth time.Time, loc *time.Location, maintenance, down []Segment, hasData bool) (month string, sla *float64, core WindowCore) {
	anchor := anyDateInMonth.In(loc)
	start := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, loc)
	end := time.Date(anchor.Year(), anchor.Month()+1, 1, 0, 0, 0, 0, loc).Add(-time.Nanosecond)

	core = DowntimeForWindow(start, end, maintenance, down, hasData)

	return start.Format("2006-01"), round4Ptr(core.SLA), core
}

// IsDown classifies one monitoring log row for SLA purposes: DOWN status,
// HTTP 5xx, or latency above the target's degraded threshold.
func IsDown(status string, httpStatusCode *int, responseTimeMs *float64, target model.Target) bool {
	if status == model.StatusDown {
		return true
	}
	if httpStatusCode != nil && *httpStatusCode >= 500 {
		return true
	}

	return responseTimeMs != nil && *responseTimeMs > float64(target.DegradedThresholdMs)
}

// ForWindow is SlaDashboardService::forWindow: DOWN segments built from
// monitoring_logs per active target, pooled cross-target SLA for one window.
// targetIDs=nil selects every active target.
func (s *SLA) ForWindow(ctx context.Context, start, end time.Time, targetIDs []string) (WindowResult, error) {
	targets, err := s.activeTargets(ctx, targetIDs)
	if err != nil {
		return WindowResult{}, err
	}

	result := WindowResult{Targets: []TargetWindow{}}
	for _, target := range targets {
		logs, err := s.logsInRange(ctx, target.ID, start, end)
		if err != nil {
			return WindowResult{}, err
		}
		segments := buildDownSegments(target, logs, end)
		hasData := len(logs) > 0

		core := DowntimeForWindow(start, end, nil, segments, hasData)
		if hasData {
			result.HasData = true
			result.TotalSeconds += core.TotalSeconds
			result.MaintenanceSeconds += core.MaintenanceSeconds
			result.DowntimeSeconds += core.DowntimeSeconds
		}
		result.Targets = append(result.Targets, TargetWindow{
			TargetID:           target.ID,
			SLA:                core.SLA,
			HasData:            core.HasData,
			DowntimeSeconds:    core.DowntimeSeconds,
			MaintenanceSeconds: core.MaintenanceSeconds,
			TotalSeconds:       core.TotalSeconds,
		})
	}

	denominator := result.TotalSeconds - result.MaintenanceSeconds
	if result.HasData && denominator > 0 {
		sla := (float64(denominator-result.DowntimeSeconds) / float64(denominator)) * 100
		result.SLA = &sla
	}

	return result, nil
}

// Calculate is SlaCalculatorService::calculate: check-count uptime, incident
// downtime, MTTR and MTBF for one target and window.
func (s *SLA) Calculate(ctx context.Context, target model.Target, start, end time.Time) (TargetSLA, error) {
	out := TargetSLA{UptimePercentage: 100}

	var totalChecks, downChecks sql.NullInt64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) AS total_checks,
		        COUNT(*) FILTER (WHERE status = 'DOWN') AS down_checks
		   FROM monitoring_logs
		  WHERE target_id = $1 AND checked_at BETWEEN $2 AND $3`,
		target.ID, start, end).Scan(&totalChecks, &downChecks)
	if err != nil {
		return TargetSLA{}, fmt.Errorf("sla checks: %w", err)
	}
	out.TotalChecks = int(totalChecks.Int64)

	var incidentCount, downtimeSeconds int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*)::int, COALESCE(SUM(duration_seconds), 0)::int
		   FROM incidents
		  WHERE target_id = $1 AND started_at BETWEEN $2 AND $3`,
		target.ID, start, end).Scan(&incidentCount, &downtimeSeconds)
	if err != nil {
		return TargetSLA{}, fmt.Errorf("sla incidents: %w", err)
	}
	out.IncidentCount = incidentCount
	out.TotalDowntimeSeconds = downtimeSeconds

	totalSeconds := int(math.Round(end.Sub(start).Seconds()))
	if out.TotalChecks > 0 {
		up := float64(out.TotalChecks - int(downChecks.Int64))
		out.UptimePercentage = round4(up / float64(out.TotalChecks) * 100)
	}

	if out.IncidentCount > 0 {
		out.MTTRSeconds = round2(float64(downtimeSeconds) / float64(out.IncidentCount))
	}
	uptimeSeconds := math.Max(0, float64(totalSeconds-downtimeSeconds))
	if out.IncidentCount > 0 {
		out.MTBFSeconds = round2(uptimeSeconds / float64(out.IncidentCount))
	} else {
		out.MTBFSeconds = round2(uptimeSeconds)
	}

	return out, nil
}

// ErrorBudget is SlaPeriodService::errorBudget: allowed downtime for a
// target fraction over a period, formatted "Xm Ys".
func ErrorBudget(targetFraction float64, periodSeconds int) (allowedSeconds float64, allowedMinutes float64, human string) {
	allowedSeconds = (1.0 - targetFraction) * float64(periodSeconds)

	return allowedSeconds, allowedSeconds / 60, FormatDuration(int(math.Round(allowedSeconds)))
}

// BudgetStatus is SlaPeriodService::status: whether the SLA target is met
// and what is left of the error budget ("Xm Ys", negative with a sign).
func BudgetStatus(sla *float64, targetFraction float64, downtimeSeconds, allowedSeconds float64) (met bool, remaining float64, human string) {
	remaining = allowedSeconds - downtimeSeconds

	return sla != nil && *sla >= targetFraction*100, remaining, FormatDuration(int(math.Round(remaining)))
}

// FormatDuration renders whole seconds as "Xm Ys" with a "-" sign prefix for
// negatives (SlaPeriodService::formatDuration).
func FormatDuration(totalSeconds int) string {
	sign := ""
	absolute := totalSeconds
	if totalSeconds < 0 {
		sign = "-"
		absolute = -totalSeconds
	}

	return fmt.Sprintf("%s%dm %ds", sign, absolute/60, absolute%60)
}

type logSample struct {
	Status         string
	HTTPStatusCode *int
	ResponseTimeMs *float64
	CheckedAt      time.Time
}

func (s *SLA) activeTargets(ctx context.Context, targetIDs []string) ([]model.Target, error) {
	query := `SELECT ` + model.Columns + `
	            FROM monitoring_targets WHERE is_active = true`
	args := []any{}
	if targetIDs != nil {
		args = append(args, targetIDs)
		query += fmt.Sprintf(" AND id = ANY($%d)", len(args))
	}
	query += " ORDER BY id"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sla targets: %w", err)
	}
	defer rows.Close()

	out := []model.Target{}
	for rows.Next() {
		target, err := model.ScanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, target)
	}

	return out, rows.Err()
}

func (s *SLA) logsInRange(ctx context.Context, targetID string, start, end time.Time) ([]logSample, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT status, http_status_code, response_time_ms::double precision, checked_at
		   FROM monitoring_logs
		  WHERE target_id = $1 AND checked_at BETWEEN $2 AND $3
		  ORDER BY checked_at`, targetID, start, end)
	if err != nil {
		return nil, fmt.Errorf("sla logs: %w", err)
	}
	defer rows.Close()

	out := []logSample{}
	for rows.Next() {
		var (
			sample  logSample
			code    sql.NullInt32
			latency sql.NullFloat64
		)
		if err := rows.Scan(&sample.Status, &code, &latency, &sample.CheckedAt); err != nil {
			return nil, err
		}
		if code.Valid {
			v := int(code.Int32)
			sample.HTTPStatusCode = &v
		}
		if latency.Valid {
			v := latency.Float64
			sample.ResponseTimeMs = &v
		}
		out = append(out, sample)
	}

	return out, rows.Err()
}

// buildDownSegments gives every DOWN-classified log the time until the next
// log (or the window end) — probe-interval discretization.
func buildDownSegments(target model.Target, logs []logSample, end time.Time) []Segment {
	segments := []Segment{}
	for i, log := range logs {
		if !IsDown(log.Status, log.HTTPStatusCode, log.ResponseTimeMs, target) {
			continue
		}
		segmentEnd := end
		if i+1 < len(logs) {
			segmentEnd = logs[i+1].CheckedAt
		}
		if segmentEnd.After(log.CheckedAt) {
			segments = append(segments, Segment{Start: log.CheckedAt, End: segmentEnd})
		}
	}

	return segments
}

func clipToWindow(segments []Segment, start, end time.Time) []Segment {
	clipped := []Segment{}
	for _, segment := range segments {
		s, e := segment.Start, segment.End
		if s.Before(start) {
			s = start
		}
		if e.After(end) {
			e = end
		}
		if !e.After(s) {
			continue
		}
		clipped = append(clipped, Segment{Start: s, End: e})
	}

	return clipped
}

// mergeSegments sorts then merges overlapping or adjacent segments so
// seconds are never double-counted before the intersection pass.
func mergeSegments(segments []Segment) []Segment {
	if len(segments) == 0 {
		return []Segment{}
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].Start.Before(segments[j].Start) })

	merged := []Segment{}
	current := segments[0]
	for _, next := range segments[1:] {
		if !next.Start.After(current.End) {
			if next.End.After(current.End) {
				current.End = next.End
			}
			continue
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)

	return merged
}

func sumSegmentSeconds(segments []Segment) float64 {
	total := 0.0
	for _, segment := range segments {
		total += math.Abs(segment.End.Sub(segment.Start).Seconds())
	}

	return total
}

// intersectionSeconds sweeps two sorted disjoint lists for overlap seconds.
func intersectionSeconds(a, b []Segment) float64 {
	seconds := 0.0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		overlapStart := a[i].Start
		if b[j].Start.After(overlapStart) {
			overlapStart = b[j].Start
		}
		overlapEnd := a[i].End
		if b[j].End.Before(overlapEnd) {
			overlapEnd = b[j].End
		}
		if overlapEnd.After(overlapStart) {
			seconds += math.Abs(overlapEnd.Sub(overlapStart).Seconds())
		}
		if a[i].End.Before(b[j].End) {
			i++
		} else {
			j++
		}
	}

	return seconds
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	out := round4(*value)

	return &out
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}
