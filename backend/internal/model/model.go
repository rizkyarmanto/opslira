// Package model holds the shared row/DTO structs used across feature
// slices. Field names and JSON tags are snake_case, 1:1 with
// resources/js/types/monitoring.d.ts.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Target status values (monitoring_targets.current_status).
const (
	StatusUp          = "UP"
	StatusDegraded    = "DEGRADED"
	StatusDown        = "DOWN"
	StatusMaintenance = "MAINTENANCE"
	StatusPending     = "PENDING"
)

// Target is a monitoring_targets row and the MonitoringTarget DTO.
type Target struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	DashboardGroup       string         `json:"dashboard_group"`
	TargetType           string         `json:"target_type"`
	Environment          string         `json:"environment"`
	HostOrEndpoint       string         `json:"host_or_endpoint"`
	Port                 *int           `json:"port"`
	ProtocolType         string         `json:"protocol_type"`
	ConfigPayload        map[string]any `json:"config_payload"`
	CheckIntervalSeconds int            `json:"check_interval_seconds"`
	TimeoutSeconds       int            `json:"timeout_seconds"`
	RetryThreshold       int            `json:"retry_threshold"`
	DegradedThresholdMs  int            `json:"degraded_threshold_ms"`
	CurrentStatus        string         `json:"current_status"`
	IsMaintenance        bool           `json:"is_maintenance"`
	IsActive             bool           `json:"is_active"`
	LastCheckedAt        *time.Time     `json:"last_checked_at"`
	NextCheckAt          time.Time      `json:"next_check_at"`
	CreatedBy            *string        `json:"-"`
	CreatedAt            time.Time      `json:"-"`
	UpdatedAt            time.Time      `json:"-"`

	// Decorated for dashboards / target detail, never stored.
	State          *TargetState `json:"state,omitempty"`
	Heartbeat      []string     `json:"heartbeat,omitempty"`
	HasCredentials *bool        `json:"has_credentials,omitempty"`
}

// TargetState is the Redis target:state:{uuid} snapshot (TargetState DTO).
type TargetState struct {
	Status              string         `json:"status"`
	LastLatencyMs       *float64       `json:"last_latency_ms"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	LastCheckedAt       *string        `json:"last_checked_at"`
	SummaryMetrics      map[string]any `json:"summary_metrics"`
}

// Incident is an incidents row and the Incident DTO.
type Incident struct {
	ID               string     `json:"id"`
	TargetID         string     `json:"target_id"`
	TargetName       *string    `json:"target_name,omitempty"`
	Severity         string     `json:"severity"`
	Status           string     `json:"status"`
	RootCauseMessage string     `json:"root_cause_message"`
	StartedAt        time.Time  `json:"started_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	DurationSeconds  *int       `json:"duration_seconds"`
}

// NocSummary is the NOC overview counter block.
type NocSummary struct {
	Total       int      `json:"total"`
	Up          int      `json:"up"`
	Degraded    int      `json:"degraded"`
	Down        int      `json:"down"`
	Maintenance int      `json:"maintenance"`
	GlobalSLA   *float64 `json:"global_sla"`
}

// NewID mints a version 7 uuid, the time-ordered shape Laravel's HasUuids
// trait wrote for every row.
func NewID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	return id.String(), nil
}
