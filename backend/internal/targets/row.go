package targets

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/model"
)

// loadTarget fetches one target by id.
func loadTarget(ctx context.Context, pool *pgxpool.Pool, id string) (model.Target, error) {
	return model.ScanTarget(pool.QueryRow(ctx, `SELECT `+model.Columns+` FROM monitoring_targets WHERE id = $1`, id))
}

// encodeObject serializes a map for a jsonb parameter.
func encodeObject(value map[string]any) []byte {
	if value == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 {
		return []byte("{}")
	}

	return raw
}

func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}

	return &v.String
}

func timePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}

	return &v.Time
}

func intPtr(v sql.NullInt32) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int32)

	return &out
}

// logRow is one monitoring_logs entry for the target detail page.
type logRow struct {
	ID             int64          `json:"id"`
	Status         string         `json:"status"`
	ResponseTimeMs *float64       `json:"response_time_ms"`
	HTTPStatusCode *int           `json:"http_status_code"`
	ErrorMessage   *string        `json:"error_message"`
	MetricsPayload map[string]any `json:"metrics_payload"`
	CheckedAt      time.Time      `json:"checked_at"`
}

func scanLogs(ctx context.Context, pool *pgxpool.Pool, targetID string) ([]logRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, status,
		        response_time_ms::double precision, http_status_code, error_message,
		        metrics_payload, checked_at
		   FROM monitoring_logs
		  WHERE target_id = $1
		  ORDER BY checked_at DESC
		  LIMIT 50`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []logRow{}
	for rows.Next() {
		var (
			l        logRow
			latency  sql.NullFloat64
			httpCode sql.NullInt32
			errMsg   sql.NullString
			metrics  []byte
		)
		if err := rows.Scan(&l.ID, &l.Status, &latency, &httpCode, &errMsg, &metrics, &l.CheckedAt); err != nil {
			return nil, err
		}
		if latency.Valid {
			v := latency.Float64
			l.ResponseTimeMs = &v
		}
		if httpCode.Valid {
			v := int(httpCode.Int32)
			l.HTTPStatusCode = &v
		}
		l.ErrorMessage = stringPtr(errMsg)
		l.MetricsPayload = model.DecodeObject(metrics)
		out = append(out, l)
	}

	return out, rows.Err()
}
