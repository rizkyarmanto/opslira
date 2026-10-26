package model

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// Columns is the canonical monitoring_targets column list matching Target's
// field order.
const Columns = `id, name, dashboard_group, target_type, environment, host_or_endpoint, port,
	protocol_type, config_payload, check_interval_seconds, timeout_seconds, retry_threshold,
	degraded_threshold_ms, current_status, is_maintenance, is_active, last_checked_at, next_check_at,
	created_by, created_at, updated_at`

// ScanTarget reads one monitoring_targets row from a pgx row or rows cursor.
// Nullable columns go through database/sql scanners, which pgx v5 supports
// natively.
func ScanTarget(row pgx.Row) (Target, error) {
	var (
		t           Target
		port        sql.NullInt32
		configRaw   []byte
		lastChecked sql.NullTime
		createdBy   sql.NullString
		createdAt   time.Time
		updatedAt   time.Time
	)
	err := row.Scan(
		&t.ID, &t.Name, &t.DashboardGroup, &t.TargetType, &t.Environment, &t.HostOrEndpoint,
		&port, &t.ProtocolType, &configRaw, &t.CheckIntervalSeconds, &t.TimeoutSeconds,
		&t.RetryThreshold, &t.DegradedThresholdMs, &t.CurrentStatus, &t.IsMaintenance, &t.IsActive,
		&lastChecked, &t.NextCheckAt, &createdBy, &createdAt, &updatedAt,
	)
	if err != nil {
		return Target{}, err
	}

	if port.Valid {
		v := int(port.Int32)
		t.Port = &v
	}
	t.ConfigPayload = DecodeObject(configRaw)
	if lastChecked.Valid {
		v := lastChecked.Time
		t.LastCheckedAt = &v
	}
	if createdBy.Valid {
		v := createdBy.String
		t.CreatedBy = &v
	}
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt

	return t, nil
}

// DecodeObject turns a jsonb column into a map; NULL/empty yields an empty
// object so config_payload never serializes as null.
func DecodeObject(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}

	return out
}
