package reporting

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"opslira/backend/internal/auth"
	"opslira/backend/internal/httpx"
	"opslira/backend/internal/uuid"
)

// Schedule is a report_schedules row and the ReportSchedule DTO (snake_case,
// 1:1 with resources/js/types/monitoring.d.ts).
type Schedule struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	PeriodType       string         `json:"period_type"`
	CronExpression   string         `json:"cron_expression"`
	Timezone         string         `json:"timezone"`
	ScopeFilter      map[string]any `json:"scope_filter"`
	ReportFormats    []string       `json:"report_formats"`
	RecipientsTo     []string       `json:"recipients_to"`
	RecipientsCC     []string       `json:"recipients_cc"`
	SendOnlyOnIssues bool           `json:"send_only_on_issues"`
	IsActive         bool           `json:"is_active"`
	LastRunAt        *time.Time     `json:"last_run_at"`
	NextRunAt        time.Time      `json:"next_run_at"`
	CreatedBy        *string        `json:"-"`
	CreatedAt        time.Time      `json:"-"`
	UpdatedAt        time.Time      `json:"-"`
}

// EmailLog is an email_delivery_logs row (EmailDeliveryLog DTO plus the
// relation columns Laravel returned).
type EmailLog struct {
	ID                  string     `json:"id"`
	EmailCategory       string     `json:"email_category"`
	GeneratedReportID   *string    `json:"generated_report_id"`
	IncidentID          *string    `json:"incident_id"`
	RecipientEmail      string     `json:"recipient_email"`
	Subject             string     `json:"subject"`
	DeliveryStatus      string     `json:"delivery_status"`
	RetryCount          int        `json:"retry_count"`
	SMTPResponseOrError *string    `json:"smtp_response_or_error"`
	SentAt              *time.Time `json:"sent_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

// GeneratedReport is a generated_reports row.
type GeneratedReport struct {
	ID               string         `json:"id"`
	ScheduleID       *string        `json:"schedule_id"`
	TriggerSource    string         `json:"trigger_source"`
	PeriodType       string         `json:"period_type"`
	PeriodStart      time.Time      `json:"period_start"`
	PeriodEnd        time.Time      `json:"period_end"`
	PDFFilePath      *string        `json:"pdf_file_path"`
	ExcelFilePath    *string        `json:"excel_file_path"`
	SummarySnapshot  map[string]any `json:"summary_snapshot"`
	GenerationStatus string         `json:"generation_status"`
	CreatedAt        time.Time      `json:"created_at"`
}

const scheduleColumns = `id, name, period_type, cron_expression, timezone, scope_filter,
	report_formats, recipients_to, recipients_cc, send_only_on_issues, is_active,
	last_run_at, next_run_at, created_by, created_at, updated_at`

func scanSchedule(row pgx.Row) (Schedule, error) {
	var (
		s          Schedule
		scopeRaw   []byte
		formatsRaw []byte
		toRaw      []byte
		ccRaw      []byte
		lastRun    sql.NullTime
		createdBy  sql.NullString
		createdAt  time.Time
		updatedAt  time.Time
	)
	err := row.Scan(&s.ID, &s.Name, &s.PeriodType, &s.CronExpression, &s.Timezone,
		&scopeRaw, &formatsRaw, &toRaw, &ccRaw, &s.SendOnlyOnIssues, &s.IsActive,
		&lastRun, &s.NextRunAt, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		return Schedule{}, err
	}

	s.ScopeFilter = decodeObject(scopeRaw)
	s.ReportFormats = decodeStringArray(formatsRaw)
	s.RecipientsTo = decodeStringArray(toRaw)
	s.RecipientsCC = decodeStringArray(ccRaw)
	s.LastRunAt = timePtrV(lastRun)
	s.CreatedBy = strPtr(createdBy)
	s.CreatedAt = createdAt
	s.UpdatedAt = updatedAt

	return s, nil
}

// scope returns the dashboard-group/environment filter of the schedule,
// defaulting to ["ALL"] exactly like the column default.
func (s Schedule) scope() (groups, environments []string) {
	groups = []string{"ALL"}
	environments = []string{"ALL"}

	if raw, ok := s.ScopeFilter["dashboard_groups"]; ok {
		if list, ok := anyToStrings(raw); ok && len(list) > 0 {
			groups = list
		}
	}
	if raw, ok := s.ScopeFilter["environments"]; ok {
		if list, ok := anyToStrings(raw); ok && len(list) > 0 {
			environments = list
		}
	}

	return groups, environments
}

// schedules lists every schedule by name (ReportController::schedules).
func (h *Handler) schedules(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+scheduleColumns+` FROM report_schedules ORDER BY name`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedules.")
		return
	}
	defer rows.Close()

	list := []Schedule{}
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedules.")
			return
		}
		list = append(list, schedule)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"schedules": list})
}

type scheduleRequest struct {
	Name             string   `json:"name"`
	PeriodType       string   `json:"period_type"`
	CronExpression   string   `json:"cron_expression"`
	Timezone         string   `json:"timezone"`
	RecipientsTo     []string `json:"recipients_to"`
	RecipientsCC     []string `json:"recipients_cc"`
	SendOnlyOnIssues *bool    `json:"send_only_on_issues"`
	ReportFormats    []string `json:"report_formats"`
	IsActive         *bool    `json:"is_active"`
}

var periodTypes = []string{"DAILY", "WEEKLY", "MONTHLY", "CUSTOM_CRON"}
var reportFormats = []string{"PDF", "XLSX", "HTML"}

func (h *Handler) store(w http.ResponseWriter, r *http.Request) {
	var req scheduleRequest
	presence, ok := httpx.DecodeWithPresence(w, r, &req)
	if !ok {
		return
	}

	errs := validateSchedule(req, presence, true)
	if len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}

	timezoneName := req.Timezone
	if timezoneName == "" {
		timezoneName = "Asia/Jakarta"
	}
	nextRun, err := NextCronRun(req.CronExpression, timezoneName, time.Now())
	if err != nil {
		// Invalid timezone: Laravel lets the exception surface as a 500.
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create schedule.")
		return
	}

	id := uuid.New()
	var createdBy *string
	if actor, ok := auth.UserFrom(r.Context()); ok {
		createdBy = &actor.ID
	}

	formatsJSON, err := json.Marshal(req.ReportFormats)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create schedule.")
		return
	}
	toJSON, _ := json.Marshal(req.RecipientsTo)
	ccJSON, _ := json.Marshal(req.RecipientsCC)

	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO report_schedules
			(id, name, period_type, cron_expression, timezone, report_formats,
			 recipients_to, recipients_cc, send_only_on_issues, is_active, next_run_at,
			 created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true,$10,$11,now(),now())`,
		id, req.Name, req.PeriodType, req.CronExpression, timezoneName, formatsJSON,
		toJSON, ccJSON, boolOrFalse(req.SendOnlyOnIssues), nextRun, createdBy)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create schedule.")
		return
	}

	schedule, err := h.loadSchedule(r.Context(), id)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedule.")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"schedule": schedule})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	if !uuid.Valid(scheduleID) {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	existing, err := h.loadSchedule(r.Context(), scheduleID)
	if err == pgx.ErrNoRows {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedule.")
		return
	}

	var req scheduleRequest
	presence, ok := httpx.DecodeWithPresence(w, r, &req)
	if !ok {
		return
	}

	errs := validateSchedule(req, presence, false)
	if len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}

	// Partial payload rules: only present keys are written, and next_run_at
	// moves only when the schedule itself moved (cron or timezone changed).
	set := []string{}
	args := []any{scheduleID}
	put := func(column, key string, value any) {
		if _, present := presence[key]; !present {
			return
		}
		args = append(args, value)
		set = append(set, column+" = $"+strconv.Itoa(len(args)))
	}
	put("name", "name", req.Name)
	put("period_type", "period_type", req.PeriodType)
	put("cron_expression", "cron_expression", req.CronExpression)
	put("timezone", "timezone", req.Timezone)
	if _, present := presence["recipients_to"]; present {
		raw, _ := json.Marshal(req.RecipientsTo)
		args = append(args, raw)
		set = append(set, "recipients_to = $"+strconv.Itoa(len(args)))
	}
	if _, present := presence["recipients_cc"]; present {
		raw, _ := json.Marshal(req.RecipientsCC)
		args = append(args, raw)
		set = append(set, "recipients_cc = $"+strconv.Itoa(len(args)))
	}
	put("send_only_on_issues", "send_only_on_issues", boolOrFalse(req.SendOnlyOnIssues))
	if _, present := presence["report_formats"]; present {
		raw, _ := json.Marshal(req.ReportFormats)
		args = append(args, raw)
		set = append(set, "report_formats = $"+strconv.Itoa(len(args)))
	}
	put("is_active", "is_active", boolOrFalse(req.IsActive))

	_, cronChanged := presence["cron_expression"]
	cronChanged = cronChanged && req.CronExpression != existing.CronExpression
	_, tzChanged := presence["timezone"]
	tzChanged = tzChanged && req.Timezone != existing.Timezone
	if cronChanged || tzChanged {
		nextRun, err := NextCronRun(
			firstNonEmpty(req.CronExpression, existing.CronExpression),
			firstNonEmpty(req.Timezone, existing.Timezone),
			time.Now())
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to update schedule.")
			return
		}
		args = append(args, nextRun)
		set = append(set, "next_run_at = $"+strconv.Itoa(len(args)))
	}

	if len(set) > 0 {
		sqlText := "UPDATE report_schedules SET " + strings.Join(set, ", ") + ", updated_at = now() WHERE id = $1"
		if _, err := h.pool.Exec(r.Context(), sqlText, args...); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to update schedule.")
			return
		}
	}

	updated, err := h.loadSchedule(r.Context(), scheduleID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedule.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"schedule": updated})
}

// destroy hard-deletes; generated_reports.schedule_id is ON DELETE SET NULL
// so report history survives (ReportController::destroy).
func (h *Handler) destroy(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	if !uuid.Valid(scheduleID) {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `DELETE FROM report_schedules WHERE id = $1`, scheduleID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to delete schedule.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	httpx.Ok(w)
}

// runNow runs generation inline and reports what actually happened
// (ReportController::runNow: 502 with ok=false on failure).
func (h *Handler) runNow(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")
	if !uuid.Valid(scheduleID) {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	schedule, err := h.loadSchedule(r.Context(), scheduleID)
	if err == pgx.ErrNoRows {
		httpx.Fail(w, http.StatusNotFound, "Schedule not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load schedule.")
		return
	}

	outcome, err := h.generator.Run(r.Context(), schedule)
	if err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]any{
			"ok":      false,
			"message": "Report generation failed: " + err.Error(),
		})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"ok":      outcome.Outcome != "failed",
		"outcome": outcome.Outcome,
		"message": outcome.Message,
		"sent":    outcome.Sent,
		"failed":  outcome.Failed,
	})
}

// emailLogs lists the newest 200 delivery logs and 50 generated reports.
func (h *Handler) emailLogs(w http.ResponseWriter, r *http.Request) {
	logRows, err := h.pool.Query(r.Context(),
		`SELECT id, email_category, generated_report_id, incident_id, recipient_email, subject,
		        delivery_status, retry_count, smtp_response_or_error, sent_at, created_at
		   FROM email_delivery_logs ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load email logs.")
		return
	}
	defer logRows.Close()

	logs := []EmailLog{}
	for logRows.Next() {
		var (
			entry      EmailLog
			reportID   sql.NullString
			incidentID sql.NullString
			smtpError  sql.NullString
			sentAt     sql.NullTime
		)
		if err := logRows.Scan(&entry.ID, &entry.EmailCategory, &reportID, &incidentID,
			&entry.RecipientEmail, &entry.Subject, &entry.DeliveryStatus, &entry.RetryCount,
			&smtpError, &sentAt, &entry.CreatedAt); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load email logs.")
			return
		}
		entry.GeneratedReportID = strPtr(reportID)
		entry.IncidentID = strPtr(incidentID)
		entry.SMTPResponseOrError = strPtr(smtpError)
		entry.SentAt = timePtrV(sentAt)
		logs = append(logs, entry)
	}

	reportRows, err := h.pool.Query(r.Context(),
		`SELECT id, schedule_id, trigger_source, period_type, period_start, period_end,
		        pdf_file_path, excel_file_path, summary_snapshot, generation_status, created_at
		   FROM generated_reports ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load reports.")
		return
	}
	defer reportRows.Close()

	reports := []GeneratedReport{}
	for reportRows.Next() {
		var (
			report     GeneratedReport
			scheduleID sql.NullString
			pdfPath    sql.NullString
			excelPath  sql.NullString
			summaryRaw []byte
		)
		if err := reportRows.Scan(&report.ID, &scheduleID, &report.TriggerSource, &report.PeriodType,
			&report.PeriodStart, &report.PeriodEnd, &pdfPath, &excelPath, &summaryRaw,
			&report.GenerationStatus, &report.CreatedAt); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load reports.")
			return
		}
		report.ScheduleID = strPtr(scheduleID)
		report.PDFFilePath = strPtr(pdfPath)
		report.ExcelFilePath = strPtr(excelPath)
		report.SummarySnapshot = decodeObject(summaryRaw)
		reports = append(reports, report)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"logs": logs, "reports": reports})
}

// download streams a generated artifact (pdf or xlsx) from the report
// storage root; missing rows or files answer 404 (ReportController::download).
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	reportID := chi.URLParam(r, "reportID")
	format := chi.URLParam(r, "format")
	if !uuid.Valid(reportID) {
		httpx.Fail(w, http.StatusNotFound, "Report not found.")
		return
	}

	var pdfPath, excelPath sql.NullString
	err := h.pool.QueryRow(r.Context(),
		`SELECT pdf_file_path, excel_file_path FROM generated_reports WHERE id = $1`,
		reportID).Scan(&pdfPath, &excelPath)
	if err == pgx.ErrNoRows {
		httpx.Fail(w, http.StatusNotFound, "Report not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load report.")
		return
	}

	relative := pdfPath
	if format == "xlsx" {
		relative = excelPath
	}
	if !relative.Valid || relative.String == "" {
		httpx.Fail(w, http.StatusNotFound, "Report file not found.")
		return
	}

	full, ok := h.storagePath(relative.String)
	if !ok {
		httpx.Fail(w, http.StatusNotFound, "Report file not found.")
		return
	}
	if _, err := os.Stat(full); err != nil {
		httpx.Fail(w, http.StatusNotFound, "Report file not found.")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(filepath.FromSlash(relative.String))+`"`)
	http.ServeFile(w, r, full)
}

func (h *Handler) loadSchedule(ctx context.Context, id string) (Schedule, error) {
	return scanSchedule(h.pool.QueryRow(ctx,
		`SELECT `+scheduleColumns+` FROM report_schedules WHERE id = $1`, id))
}

// validateSchedule implements ReportController's rules; on update every rule
// is gated on the key being present ('sometimes').
func validateSchedule(req scheduleRequest, presence map[string]json.RawMessage, create bool) httpx.ValidationErrors {
	errs := httpx.ValidationErrors{}
	present := func(key string) bool {
		_, ok := presence[key]
		return ok
	}

	if create || present("name") {
		if req.Name == "" {
			errs["name"] = append(errs["name"], "The name field is required.")
		} else if len(req.Name) > 150 {
			errs["name"] = append(errs["name"], "The name may not be greater than 150 characters.")
		}
	}
	if create || present("period_type") {
		if req.PeriodType == "" && create {
			errs["period_type"] = append(errs["period_type"], "The period_type field is required.")
		} else if !containsString(periodTypes, req.PeriodType) {
			errs["period_type"] = append(errs["period_type"], "The selected period_type is invalid.")
		}
	}
	if create || present("cron_expression") {
		switch {
		case req.CronExpression == "":
			errs["cron_expression"] = append(errs["cron_expression"], "The cron_expression field is required.")
		case len(req.CronExpression) > 60:
			errs["cron_expression"] = append(errs["cron_expression"], "The cron_expression may not be greater than 60 characters.")
		default:
			if err := ValidCron(req.CronExpression); err != nil {
				errs["cron_expression"] = append(errs["cron_expression"], cronInvalidMessage)
			}
		}
	}
	if create || present("timezone") {
		if len(req.Timezone) > 50 {
			errs["timezone"] = append(errs["timezone"], "The timezone may not be greater than 50 characters.")
		}
	}
	if create || present("report_formats") {
		if len(req.ReportFormats) == 0 {
			errs["report_formats"] = append(errs["report_formats"], "The report_formats field must have at least 1 element.")
		} else {
			for i, format := range req.ReportFormats {
				if !containsString(reportFormats, format) {
					errs["report_formats"] = append(errs["report_formats"],
						"The selected report_formats."+strconv.Itoa(i)+" is invalid.")
				}
			}
		}
	}
	if present("recipients_to") {
		validateEmailList(req.RecipientsTo, "recipients_to", errs)
	}
	if present("recipients_cc") {
		validateEmailList(req.RecipientsCC, "recipients_cc", errs)
	}

	return errs
}

func validateEmailList(list []string, field string, errs httpx.ValidationErrors) {
	for i, value := range list {
		if !isEmail(value) {
			errs[field] = append(errs[field],
				"The "+field+"."+strconv.Itoa(i)+" must be a valid email address.")
		}
	}
}

func isEmail(value string) bool {
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	local, domain := value[:at], value[at+1:]
	if strings.ContainsAny(local, " \t<>") || strings.Contains(domain, " ") {
		return false
	}
	return strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".") &&
		!strings.HasPrefix(domain, ".") && !strings.Contains(domain, "..")
}

func boolOrFalse(value *bool) bool { return value != nil && *value }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
