// Package incidents opens/resolves incident rows on probe state changes and
// dispatches the matching alerts (email delivery logs + SMTP + Telegram +
// Slack), inline — no queue worker consumes anything after the Go split.
package incidents

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/config"
	"opslira/backend/internal/mailer"
	"opslira/backend/internal/model"
	"opslira/backend/internal/uuid"
)

// Service implements the ProcessStateChangeAlertJob + incident persistence
// used by the probe execution pipeline.
type Service struct {
	pool   *pgxpool.Pool
	mailer *mailer.Mailer
	cfg    config.Config
	logger *slog.Logger
}

// New wires the incident service.
func New(pool *pgxpool.Pool, m *mailer.Mailer, cfg config.Config, logger *slog.Logger) *Service {
	return &Service{pool: pool, mailer: m, cfg: cfg, logger: logger}
}

// Open creates a CRITICAL/WARNING incident for a state change and returns
// its id (the value ProbeExecutionService stored inline with the DOWN state).
func (s *Service) Open(ctx context.Context, t model.Target, severity, rootCause string) (string, error) {
	if severity == "" {
		severity = "CRITICAL"
	}
	if strings.TrimSpace(rootCause) == "" {
		rootCause = "Target unreachable"
	}

	id := uuid.New()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO incidents (id, target_id, severity, status, root_cause_message, started_at, created_at)
		 VALUES ($1, $2, $3, 'OPEN', $4, now(), now())`,
		id, t.ID, severity, rootCause)
	if err != nil {
		return "", fmt.Errorf("open incident: %w", err)
	}

	return id, nil
}

// ResolveOpen marks every non-RESOLVED incident of a target as RESOLVED with
// a computed duration (mirrors ProbeExecutionService::resolveOpenIncidents).
func (s *Service) ResolveOpen(ctx context.Context, targetID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE incidents
		    SET status = 'RESOLVED',
		        resolved_at = now(),
		        duration_seconds = GREATEST(0, round(EXTRACT(EPOCH FROM (now() - started_at))))::int
		  WHERE target_id = $1 AND status <> 'RESOLVED'`, targetID)
	if err != nil {
		return fmt.Errorf("resolve incidents: %w", err)
	}

	return nil
}

// Notify sends the DOWN/RECOVERED alert: one email_delivery_logs row per
// recipient (SENT/FAILED around the SMTP attempt) plus best-effort
// Telegram/Slack posts — AlertNotificationService ported inline.
func (s *Service) Notify(ctx context.Context, t model.Target, event, incidentID string) {
	subject := s.buildSubject(t, event)
	body := s.buildBody(ctx, t, event)

	recipients := s.cfg.AlertRecipients
	if len(recipients) == 0 {
		s.logger.Warn("No alert recipients configured, skipping incident email")
	}
	for _, recipient := range recipients {
		s.sendAlertEmail(ctx, t.ID, incidentID, recipient, subject, body)
	}

	s.sendTelegram(ctx, body)
	s.sendSlack(ctx, body)
}

func (s *Service) sendAlertEmail(ctx context.Context, targetID, incidentID, recipient, subject, body string) {
	var storedIncidentID *string
	if incidentID != "" {
		storedIncidentID = &incidentID
	}

	logID := uuid.New()
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO email_delivery_logs
			(id, email_category, incident_id, recipient_email, subject, delivery_status, retry_count, created_at)
		 VALUES ($1, 'INCIDENT_ALERT', $2, $3, $4, 'PENDING', 0, now())`,
		logID, storedIncidentID, recipient, subject); err != nil {
		s.logger.Error("Alert delivery log insert failed", "error", err, "target_id", targetID)
		return
	}

	err := s.mailer.Send(ctx, []string{recipient}, subject, body)
	if err == nil {
		_, updateErr := s.pool.Exec(ctx,
			`UPDATE email_delivery_logs SET delivery_status = 'SENT', sent_at = now() WHERE id = $1`, logID)
		if updateErr != nil {
			s.logger.Error("Alert delivery log update failed", "error", updateErr)
		}
		return
	}

	_, updateErr := s.pool.Exec(ctx,
		`UPDATE email_delivery_logs
		    SET delivery_status = 'FAILED',
		        retry_count = retry_count + 1,
		        smtp_response_or_error = $2
		  WHERE id = $1`, logID, err.Error())
	if updateErr != nil {
		s.logger.Error("Alert delivery log update failed", "error", updateErr)
	}
	s.logger.Error("Alert email failed", "to", recipient, "error", err.Error())
}

func (s *Service) buildSubject(t model.Target, event string) string {
	prefix := "[ALERT]"
	switch event {
	case "DOWN":
		prefix = "[CRITICAL] DOWN"
	case "RECOVERED":
		prefix = "[RESOLVED] RECOVERED"
	}

	return prefix + ": " + t.Name
}

func (s *Service) buildBody(ctx context.Context, t model.Target, event string) string {
	emoji := "🟢"
	if event == "DOWN" {
		emoji = "🔴"
	}

	endpoint := t.HostOrEndpoint
	if t.Port != nil {
		endpoint = fmt.Sprintf("%s:%d", endpoint, *t.Port)
	}
	loc := s.cfg.Location()
	lines := []string{
		fmt.Sprintf("%s Status change: %s", emoji, event),
		"Target: " + t.Name,
		"Endpoint: " + endpoint,
		fmt.Sprintf("Type: %s (%s)", t.TargetType, t.ProtocolType),
		"Environment: " + t.Environment,
		"Time: " + time.Now().In(loc).Format("2006-01-02 15:04:05") + " WIB",
	}

	if event == "DOWN" {
		var cause string
		err := s.pool.QueryRow(ctx,
			`SELECT root_cause_message FROM incidents
			  WHERE target_id = $1 AND status <> 'RESOLVED'
			  ORDER BY started_at DESC LIMIT 1`, t.ID).Scan(&cause)
		if err == nil && cause != "" {
			lines = append(lines, "Cause: "+cause)
		}
	}

	return strings.Join(lines, "\n")
}

func (s *Service) sendTelegram(ctx context.Context, body string) {
	token := s.cfg.TelegramBotToken
	chatID := s.cfg.TelegramChatID
	if token == "" || chatID == "" {
		return
	}

	endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"
	form := url.Values{"chat_id": {chatID}, "text": {body}}
	if err := postForm(ctx, endpoint, form); err != nil {
		s.logger.Warn("Telegram alert failed", "error", err.Error())
	}
}

func (s *Service) sendSlack(ctx context.Context, body string) {
	if s.cfg.SlackWebhookURL == "" {
		return
	}
	form := url.Values{"text": {body}}
	if err := postForm(ctx, s.cfg.SlackWebhookURL, form); err != nil {
		s.logger.Warn("Slack alert failed", "error", err.Error())
	}
}

func postForm(ctx context.Context, endpoint string, form url.Values) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}
