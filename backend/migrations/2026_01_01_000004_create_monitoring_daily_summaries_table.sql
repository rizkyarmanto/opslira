-- Transcribed from database/migrations/2026_01_01_000004_create_monitoring_daily_summaries_table.php
CREATE TABLE IF NOT EXISTS monitoring_daily_summaries (
    id BIGSERIAL PRIMARY KEY,
    target_id UUID NOT NULL REFERENCES monitoring_targets (id) ON DELETE CASCADE,
    summary_date DATE NOT NULL,
    total_checks INTEGER NOT NULL DEFAULT 0,
    successful_checks INTEGER NOT NULL DEFAULT 0,
    failed_checks INTEGER NOT NULL DEFAULT 0,
    uptime_percentage NUMERIC(7, 4) NOT NULL DEFAULT 100.0000,
    avg_response_time_ms NUMERIC(10, 2) NOT NULL DEFAULT 0,
    max_response_time_ms NUMERIC(10, 2) NOT NULL DEFAULT 0,
    p95_response_time_ms NUMERIC(10, 2) NOT NULL DEFAULT 0,
    total_incidents INTEGER NOT NULL DEFAULT 0,
    total_downtime_seconds INTEGER NOT NULL DEFAULT 0,
    maintenance_seconds INTEGER NOT NULL DEFAULT 0,
    daily_metrics_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (target_id, summary_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_summaries_date_target ON monitoring_daily_summaries (summary_date DESC, target_id);
