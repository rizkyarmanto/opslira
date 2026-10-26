-- Transcribed from database/migrations/2026_01_01_000003_create_monitoring_logs_table.php
-- Declarative range partitioning by checked_at (monthly).
CREATE TABLE IF NOT EXISTS monitoring_logs (
    id BIGSERIAL,
    target_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('UP', 'DEGRADED', 'DOWN')),
    response_time_ms NUMERIC(10, 2),
    http_status_code INTEGER,
    error_message TEXT,
    metrics_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, checked_at)
) PARTITION BY RANGE (checked_at);

CREATE INDEX IF NOT EXISTS idx_monitoring_logs_target_time ON monitoring_logs (target_id, checked_at DESC);

-- Current month + next 2 months of partitions (the scheduler's prune job
-- keeps the rolling window alive afterwards).
DO $$
DECLARE
    month_start DATE;
    month_end DATE;
    suffix TEXT;
BEGIN
    FOR i IN 0..2 LOOP
        month_start := date_trunc('month', CURRENT_TIMESTAMP + make_interval(months => i))::date;
        month_end := (month_start + INTERVAL '1 month')::date;
        suffix := to_char(month_start, 'YYYY_MM');
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS monitoring_logs_%s PARTITION OF monitoring_logs FOR VALUES FROM (%L) TO (%L)',
            suffix, month_start, month_end
        );
    END LOOP;
END $$;
