package probes

import (
	"context"
	"log/slog"
	"time"

	"opslira/backend/internal/model"
	"opslira/backend/internal/probestate"
)

// IncidentStore persists the side effects of a probe: the raw
// monitoring_logs row, the target status/heartbeat columns, and the incidents
// rows that ProbeExecutionService writes in PHP. Implementations must be safe
// for concurrent use by the scheduler worker pool.
type IncidentStore interface {
	// InsertLog writes the raw monitoring_logs row (partitioned table, so no ORM).
	InsertLog(ctx context.Context, targetID string, result ProbeResult, checkedAt time.Time) error
	// UpdateTargetStatus sets current_status + last_checked_at (+ updated_at).
	UpdateTargetStatus(ctx context.Context, t model.Target, status string, checkedAt time.Time) error
	// TouchTarget advances last_checked_at only (a failing check below threshold).
	TouchTarget(ctx context.Context, t model.Target, checkedAt time.Time) error
	// OpenIncident creates the OPEN CRITICAL incident and returns its id.
	OpenIncident(ctx context.Context, t model.Target, rootCause string, startedAt time.Time) (string, error)
	// ResolveOpenIncidents closes every non-RESOLVED incident with duration.
	ResolveOpenIncidents(ctx context.Context, t model.Target, resolvedAt time.Time) (int, error)
}

// ExecutionService orchestrates a single probe end-to-end:
// adapter -> monitoring_logs insert -> Redis state -> state-change/incident
// logic (ProbeExecutionService).
type ExecutionService struct {
	Factory   Factory
	State     *probestate.Service
	Decryptor CredentialDecryptor // nil disables decryption (targets without credentials still run)
	Incidents IncidentStore       // nil skips SQL side effects (tests / not wired yet)
	Alert     StateChangeHook     // nil skips notifications
	Log       *slog.Logger
}

func (s *ExecutionService) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// Execute runs one probe end-to-end. An unknown protocol_type surfaces as
// ErrUnknownProtocol; probe failures are DOWN values in the result.
func (s *ExecutionService) Execute(ctx context.Context, t model.Target, cred *Credential) (ProbeResult, error) {
	// ExecuteTargetProbeJob re-checks after the claim: the row can be disabled
	// or moved to maintenance between claim and run, and then nothing is logged.
	if !t.IsActive || t.IsMaintenance {
		return ProbeResult{}, nil
	}

	adapter, err := s.Factory.Make(t.ProtocolType)
	if err != nil {
		return ProbeResult{}, err
	}

	// Same config assembly as PHP: config_payload + injected host + port.
	config := make(map[string]any, len(t.ConfigPayload)+2)
	for key, value := range t.ConfigPayload {
		config[key] = value
	}
	config["host"] = t.HostOrEndpoint
	config["port"] = t.Port

	if tuned, ok := adapter.(TunedAdapter); ok {
		adapter = tuned.Tune(t.TimeoutSeconds, t.DegradedThresholdMs)
	}
	adapter.Configure(config, s.decryptCredentials(ctx, t, cred))

	result := adapter.Execute(ctx)

	if s.Incidents != nil {
		if err := s.Incidents.InsertLog(ctx, t.ID, result, time.Now()); err != nil {
			s.logger().ErrorContext(ctx, "monitoring_logs insert failed", "target_id", t.ID, "err", err)
		}
	}
	LogFailure(ctx, s.logger(), t, result, "scheduled")
	s.applyStateChange(ctx, t, result)

	return result, nil
}

// decryptCredentials mirrors PHP: a failed decrypt logs a warning and the
// probe continues with empty credentials.
func (s *ExecutionService) decryptCredentials(ctx context.Context, t model.Target, cred *Credential) map[string]any {
	if cred == nil || cred.EncryptedPayload == "" || s.Decryptor == nil {
		return nil
	}
	payload, err := s.Decryptor.Decrypt(cred.EncryptedPayload, cred.EncryptionIV, cred.EncryptionTag)
	if err != nil {
		s.logger().WarnContext(ctx, "Credential decryption failed", "target_id", t.ID, "error", err)
		return nil
	}
	return payload
}

// applyStateChange ports ProbeExecutionService::applyStateChange: DOWN only
// after retry_threshold consecutive failures, recovery resolves open
// incidents, notifications fire inline.
func (s *ExecutionService) applyStateChange(ctx context.Context, t model.Target, result ProbeResult) {
	previous := s.State.PreviousStatus(ctx, t.ID)

	if result.IsDown() {
		failures := s.State.IncrementFailures(ctx, t.ID)
		threshold := t.RetryThreshold
		if threshold <= 0 {
			threshold = 3 // schema default; guards zero-value structs in tests
		}

		// DOWN only after retry_threshold consecutive failures.
		if failures >= threshold && previous != model.StatusDown {
			s.State.SetStatus(ctx, t.ID, model.StatusDown, result.ResponseTimeMs, result.MetricsPayload)
			now := time.Now()
			if s.Incidents == nil {
				return
			}
			if err := s.Incidents.UpdateTargetStatus(ctx, t, model.StatusDown, now); err != nil {
				s.logger().ErrorContext(ctx, "target status update failed", "target_id", t.ID, "err", err)
			}
			cause := "Target unreachable"
			if result.ErrorMessage != nil && *result.ErrorMessage != "" {
				cause = *result.ErrorMessage
			}
			incidentID, err := s.Incidents.OpenIncident(ctx, t, cause, now)
			if err != nil {
				s.logger().ErrorContext(ctx, "incident insert failed", "target_id", t.ID, "err", err)
				return
			}
			if s.Alert != nil {
				s.Alert(ctx, t, EventDown, incidentID)
			}
			return
		}

		if s.Incidents != nil {
			if err := s.Incidents.TouchTarget(ctx, t, time.Now()); err != nil {
				s.logger().ErrorContext(ctx, "target heartbeat update failed", "target_id", t.ID, "err", err)
			}
		}
		return
	}

	// Success path (UP or DEGRADED): failures reset, status stored.
	s.State.ResetFailures(ctx, t.ID)
	s.State.SetStatus(ctx, t.ID, result.Status, result.ResponseTimeMs, result.MetricsPayload)
	if s.Incidents != nil {
		if err := s.Incidents.UpdateTargetStatus(ctx, t, result.Status, time.Now()); err != nil {
			s.logger().ErrorContext(ctx, "target status update failed", "target_id", t.ID, "err", err)
		}
	}

	// Recovery: previous was DOWN and now the probe is strictly UP.
	if previous == model.StatusDown && result.IsUp() {
		if s.Incidents != nil {
			if _, err := s.Incidents.ResolveOpenIncidents(ctx, t, time.Now()); err != nil {
				s.logger().ErrorContext(ctx, "incident resolve failed", "target_id", t.ID, "err", err)
			}
		}
		if s.Alert != nil {
			s.Alert(ctx, t, EventRecovered, "")
		}
	}
}
