-- Transcribed from database/migrations/2026_01_01_000006_create_report_schedules_table.php
CREATE TABLE IF NOT EXISTS report_schedules (
    id UUID PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    period_type VARCHAR(20) NOT NULL,
    cron_expression VARCHAR(60) NOT NULL,
    timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    scope_filter JSONB NOT NULL DEFAULT '{"dashboard_groups": ["ALL"], "environments": ["ALL"]}'::jsonb,
    report_formats JSONB NOT NULL DEFAULT '["HTML", "PDF", "XLSX"]'::jsonb,
    recipients_to JSONB NOT NULL DEFAULT '[]'::jsonb,
    recipients_cc JSONB NOT NULL DEFAULT '[]'::jsonb,
    send_only_on_issues BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'rs_period_type_check') THEN ALTER TABLE report_schedules ADD CONSTRAINT rs_period_type_check
    CHECK (period_type IN ('DAILY', 'WEEKLY', 'MONTHLY', 'CUSTOM_CRON')); END IF; END $$;

CREATE INDEX IF NOT EXISTS idx_report_schedules_next_run ON report_schedules (is_active, next_run_at);
