package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/config"
	"opslira/backend/internal/mailer"
	"opslira/backend/internal/model"
	"opslira/backend/internal/reportarchive"
	"opslira/backend/internal/reportrender"
	"opslira/backend/internal/uuid"
)

// Outcome is the run result surfaced by run-now and the report tick.
type Outcome struct {
	OK      bool   `json:"ok"`
	Outcome string `json:"outcome"` // created | skipped | failed
	Message string `json:"message"`
	Sent    *int   `json:"sent"`
	Failed  *int   `json:"failed"`
}

// Generator produces one report per schedule period and emails it —
// GenerateAndSendReportJob ported to run inline.
type Generator struct {
	pool     *pgxpool.Pool
	sla      *SLA
	mailer   *mailer.Mailer
	cfg      config.Config
	logger   *slog.Logger
	archiver *reportarchive.Archiver
}

// NewGenerator wires report generation.
func NewGenerator(pool *pgxpool.Pool, sla *SLA, m *mailer.Mailer, cfg config.Config, logger *slog.Logger) *Generator {
	return &Generator{pool: pool, sla: sla, mailer: m, cfg: cfg, logger: logger, archiver: reportarchive.New(cfg.ReportS3)}
}

// Run generates the schedule's current period report. It returns the
// created/skipped outcome, or an error after marking the report row FAILED.
func (g *Generator) Run(ctx context.Context, schedule Schedule) (Outcome, error) {
	loc := timezoneOr(schedule.Timezone, g.cfg.Location())
	periodStart, periodEnd := resolvePeriod(schedule.PeriodType, time.Now().In(loc))

	// Idempotency: one report per (schedule, period_start); COMPLETED rows
	// skip, failed rows stay retryable and are reused instead of duplicated.
	var existingID string
	err := g.pool.QueryRow(ctx,
		`SELECT id FROM generated_reports
		  WHERE schedule_id = $1 AND period_start = $2 AND generation_status = 'COMPLETED'
		  LIMIT 1`, schedule.ID, periodStart).Scan(&existingID)
	if err == nil {
		return Outcome{
			OK:      true,
			Outcome: "skipped",
			Message: "A report for this period already exists; it was not regenerated.",
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, fmt.Errorf("report lookup: %w", err)
	}

	reportID := uuid.New()
	reused := false
	err = g.pool.QueryRow(ctx,
		`SELECT id FROM generated_reports
		  WHERE schedule_id = $1 AND period_start = $2 AND generation_status <> 'COMPLETED'
		  LIMIT 1`, schedule.ID, periodStart).Scan(&existingID)
	switch {
	case err == nil:
		reportID = existingID
		reused = true
	case !errors.Is(err, pgx.ErrNoRows):
		return Outcome{}, fmt.Errorf("report lookup: %w", err)
	}

	storedPeriod := schedule.PeriodType
	if storedPeriod == "CUSTOM_CRON" {
		storedPeriod = "DAILY"
	}

	if reused {
		if _, err := g.pool.Exec(ctx,
			`UPDATE generated_reports SET generation_status = 'PROCESSING', trigger_source = 'MANUAL_RUN_NOW' WHERE id = $1`,
			reportID); err != nil {
			return Outcome{}, fmt.Errorf("report update: %w", err)
		}
	} else {
		if _, err := g.pool.Exec(ctx,
			`INSERT INTO generated_reports
				(id, schedule_id, trigger_source, period_type, period_start, period_end, generation_status, created_at)
			 VALUES ($1, $2, 'SCHEDULED', $3, $4, $5, 'PROCESSING', now())`,
			reportID, schedule.ID, storedPeriod, periodStart, periodEnd); err != nil {
			return Outcome{}, fmt.Errorf("report insert: %w", err)
		}
	}

	failed := func(cause error) (Outcome, error) {
		if _, err := g.pool.Exec(ctx,
			`UPDATE generated_reports SET generation_status = 'FAILED' WHERE id = $1`, reportID); err != nil {
			g.logger.Error("Failed to mark report FAILED", "report_id", reportID, "error", err)
		}
		return Outcome{}, cause
	}

	data, err := g.buildData(ctx, schedule, periodStart, periodEnd)
	if err != nil {
		return failed(fmt.Errorf("Report data build failed: %w", err))
	}

	if schedule.SendOnlyOnIssues && data.Summary.TotalIncidents == 0 {
		if err := g.complete(ctx, reportID, data); err != nil {
			return failed(err)
		}

		return Outcome{
			OK:      true,
			Outcome: "skipped",
			Message: "No incidents in this period; the report was not sent.",
		}, nil
	}

	if containsFormat(schedule.ReportFormats, "PDF") {
		html, err := reportrender.RenderHTML(data)
		if err != nil {
			return failed(fmt.Errorf("Report template render failed: %w", err))
		}
		pdf, err := reportrender.BuildPDF(ctx, g.cfg.GotenbergURL, html)
		if err != nil {
			return failed(fmt.Errorf("PDF build failed: %w", err))
		}
		rel, err := g.storeFile(periodStart, storedPeriod, "pdf", pdf)
		if err != nil {
			return failed(err)
		}
		if _, err := g.pool.Exec(ctx,
			`UPDATE generated_reports SET pdf_file_path = $2 WHERE id = $1`, reportID, rel); err != nil {
			return failed(fmt.Errorf("report update: %w", err))
		}
		g.archive(ctx, reportID, rel, pdf, "application/pdf")
	}

	if containsFormat(schedule.ReportFormats, "XLSX") {
		xlsx, err := reportrender.BuildXLSX(data)
		if err != nil {
			return failed(fmt.Errorf("Excel build failed: %w", err))
		}
		rel, err := g.storeFile(periodStart, storedPeriod, "xlsx", xlsx)
		if err != nil {
			return failed(err)
		}
		if _, err := g.pool.Exec(ctx,
			`UPDATE generated_reports SET excel_file_path = $2 WHERE id = $1`, reportID, rel); err != nil {
			return failed(fmt.Errorf("report update: %w", err))
		}
		g.archive(ctx, reportID, rel, xlsx, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}

	if err := g.complete(ctx, reportID, data); err != nil {
		return failed(err)
	}

	sent, sendFailed, err := g.sendEmails(ctx, schedule, reportID, data)
	if err != nil {
		return failed(fmt.Errorf("Email delivery step failed: %w", err))
	}

	switch {
	case sent == 0 && sendFailed == 0:
		return Outcome{
			OK:      true,
			Outcome: "created",
			Message: "Report created, no recipients configured (not sent).",
			Sent:    new(0),
			Failed:  new(0),
		}, nil
	case sendFailed > 0:
		return Outcome{
			OK:      true,
			Outcome: "created",
			Message: fmt.Sprintf("Report created. Email: %d sent, %d failed.", sent, sendFailed),
			Sent:    new(sent),
			Failed:  new(sendFailed),
		}, nil
	default:
		return Outcome{
			OK:      true,
			Outcome: "created",
			Message: "Report created.",
			Sent:    new(sent),
			Failed:  new(sendFailed),
		}, nil
	}
}

// buildData assembles the template/Excel input from scope-filtered targets
// (GenerateAndSendReportJob::buildReportData).
func (g *Generator) buildData(ctx context.Context, schedule Schedule, start, end time.Time) (reportrender.Data, error) {
	groups, environments := schedule.scope()

	query := `SELECT id, name, dashboard_group, target_type, environment
	            FROM monitoring_targets WHERE is_active = true`
	args := []any{}
	if !containsString(groups, "ALL") {
		args = append(args, groups)
		query += fmt.Sprintf(" AND dashboard_group = ANY($%d)", len(args))
	}
	if !containsString(environments, "ALL") {
		args = append(args, environments)
		query += fmt.Sprintf(" AND environment = ANY($%d)", len(args))
	}
	query += " ORDER BY name"

	rows, err := g.pool.Query(ctx, query, args...)
	if err != nil {
		return reportrender.Data{}, fmt.Errorf("report targets: %w", err)
	}
	defer rows.Close()

	data := reportrender.Data{
		PeriodType:   schedule.PeriodType,
		PeriodLabel:  start.Format("02 Jan 2006") + " to " + end.Format("02 Jan 2006"),
		GeneratedAt:  time.Now().In(g.cfg.Location()).Format("2006-01-02 15:04:05") + " WIB",
		ScheduleName: schedule.Name,
		Rows:         []reportrender.Row{},
	}
	if data.PeriodType == "CUSTOM_CRON" {
		data.PeriodType = "DAILY"
	}

	for rows.Next() {
		var id, name, group, targetType, environment string
		if err := rows.Scan(&id, &name, &group, &targetType, &environment); err != nil {
			return reportrender.Data{}, fmt.Errorf("report target scan: %w", err)
		}

		sla, err := g.sla.Calculate(ctx, model.Target{ID: id, Name: name}, start, end)
		if err != nil {
			return reportrender.Data{}, err
		}

		data.Rows = append(data.Rows, reportrender.Row{
			Name:                 name,
			DashboardGroup:       group,
			Environment:          environment,
			TargetType:           targetType,
			UptimePercentage:     sla.UptimePercentage,
			MTTRSeconds:          sla.MTTRSeconds,
			MTBFSeconds:          sla.MTBFSeconds,
			TotalDowntimeSeconds: sla.TotalDowntimeSeconds,
			IncidentCount:        sla.IncidentCount,
			TotalChecks:          sla.TotalChecks,
		})

		data.Summary.TotalIncidents += sla.IncidentCount
		data.Summary.TotalDowntimeSeconds += sla.TotalDowntimeSeconds
	}
	if err := rows.Err(); err != nil {
		return reportrender.Data{}, err
	}

	if len(data.Rows) > 0 {
		slaSum := 0.0
		for _, row := range data.Rows {
			slaSum += row.UptimePercentage
		}
		data.Summary.GlobalSLA = round2(slaSum / float64(len(data.Rows)))
	} else {
		data.Summary.GlobalSLA = 100
	}
	data.Summary.TotalTargets = len(data.Rows)
	if data.Summary.TotalIncidents > 0 {
		data.Summary.MTTRSeconds = round2(float64(data.Summary.TotalDowntimeSeconds) / float64(data.Summary.TotalIncidents))
	} else {
		data.Summary.MTTRSeconds = 0
	}

	return data, nil
}

// complete writes COMPLETED plus the summary snapshot.
func (g *Generator) complete(ctx context.Context, reportID string, data reportrender.Data) error {
	summary, err := json.Marshal(data.Summary)
	if err != nil {
		return err
	}
	_, err = g.pool.Exec(ctx,
		`UPDATE generated_reports SET generation_status = 'COMPLETED', summary_snapshot = $2 WHERE id = $1`,
		reportID, summary)

	return err
}

// sendEmails logs one delivery row per recipient (TO + CC) and attempts
// SMTP per recipient; a recipient failure never aborts the loop.
func (g *Generator) sendEmails(ctx context.Context, schedule Schedule, reportID string, data reportrender.Data) (sent, failed int, err error) {
	recipients := append([]string{}, schedule.RecipientsTo...)
	recipients = append(recipients, schedule.RecipientsCC...)

	subject := fmt.Sprintf("[Opslira] %s report, %s", data.PeriodType, schedule.Name)
	body := fmt.Sprintf("Report %s\nGlobal SLA: %v%%\nIncidents: %d",
		data.PeriodLabel, data.Summary.GlobalSLA, data.Summary.TotalIncidents)

	for _, recipient := range recipients {
		logID := uuid.New()
		if _, err := g.pool.Exec(ctx,
			`INSERT INTO email_delivery_logs
				(id, email_category, generated_report_id, recipient_email, subject, delivery_status, retry_count, created_at)
			 VALUES ($1, 'SCHEDULED_REPORT', $2, $3, $4, 'PENDING', 0, now())`,
			logID, reportID, recipient, subject); err != nil {
			return sent, failed, fmt.Errorf("delivery log insert: %w", err)
		}

		sendErr := g.mailer.Send(ctx, []string{recipient}, subject, body)
		if sendErr == nil {
			if _, err := g.pool.Exec(ctx,
				`UPDATE email_delivery_logs SET delivery_status = 'SENT', sent_at = now() WHERE id = $1`, logID); err != nil {
				return sent, failed, fmt.Errorf("delivery log update: %w", err)
			}
			sent++
			continue
		}

		if _, err := g.pool.Exec(ctx,
			`UPDATE email_delivery_logs
			    SET delivery_status = 'FAILED', retry_count = retry_count + 1, smtp_response_or_error = $2
			  WHERE id = $1`, logID, sendErr.Error()); err != nil {
			return sent, failed, fmt.Errorf("delivery log update: %w", err)
		}
		failed++
		g.logger.Error("Report email failed", "to", recipient, "error", sendErr.Error())
	}

	return sent, failed, nil
}

// storeFile writes artifact content under
// REPORT_STORAGE_DIR/reports/YYYY-MM/<PERIOD>-Ymd-His.<ext> and returns the
// stored relative path (forward slashes, Laravel-style).
func (g *Generator) storeFile(periodStart time.Time, periodType, ext string, content []byte) (string, error) {
	directory := "reports/" + periodStart.Format("2006-01")
	name := periodType + "-" + time.Now().Format("20060102-150405") + "." + ext

	full := filepath.Join(g.cfg.ReportStorageDir, filepath.FromSlash(directory))
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", fmt.Errorf("report dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(full, name), content, 0o644); err != nil {
		return "", fmt.Errorf("report write: %w", err)
	}

	return directory + "/" + name, nil
}

// archive uploads one artifact to the S3 archive sink. Best-effort: skipped
// when REPORT_S3_ENDPOINT is empty, and a failure only logs — the local
// file stays the source of truth and the report still completes.
func (g *Generator) archive(ctx context.Context, reportID, rel string, content []byte, contentType string) {
	if g.archiver == nil {
		return
	}
	key, err := g.archiver.Put(ctx, rel, content, contentType)
	if err != nil {
		g.logger.Warn("Report archive upload failed", "report_id", reportID, "key", rel, "error", err.Error())
		return
	}
	g.logger.Info("Report archived to S3", "report_id", reportID, "key", key)
}

// resolvePeriod mirrors GenerateAndSendReportJob::resolvePeriod: the window
// ends now, starting at the previous day/week/month boundary in the
// schedule zone (startOfDay for every period type).
func resolvePeriod(periodType string, now time.Time) (start, end time.Time) {
	end = now
	switch periodType {
	case "WEEKLY":
		start = startOfDay(end.AddDate(0, 0, -7))
	case "MONTHLY":
		// subMonthNoOverflow: same day-of-month, clamped (Mar 31 -> Feb 28).
		year, month := end.Year(), end.Month()-1
		if month == 0 {
			month = time.December
			year--
		}
		day := end.Day()
		if last := time.Date(year, month+1, 1, 0, 0, 0, 0, end.Location()).Add(-time.Second).Day(); day > last {
			day = last
		}
		start = time.Date(year, month, day, 0, 0, 0, 0, end.Location())
	default:
		start = startOfDay(end.AddDate(0, 0, -1))
	}

	return start, end
}

func containsFormat(formats []string, wanted string) bool { return containsString(formats, wanted) }
