-- Transcribed from database/migrations/2026_01_01_000007_create_generated_reports_table.php
CREATE TABLE IF NOT EXISTS generated_reports (
    id UUID PRIMARY KEY,
    schedule_id UUID REFERENCES report_schedules (id) ON DELETE SET NULL,
    trigger_source VARCHAR(20) NOT NULL,
    period_type VARCHAR(20) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    pdf_file_path VARCHAR(255),
    excel_file_path VARCHAR(255),
    summary_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    generation_status VARCHAR(20) NOT NULL DEFAULT 'PROCESSING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Idempotency guard: one report per (schedule, period_start)
    UNIQUE (schedule_id, period_start)
);

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'gr_trigger_source_check') THEN ALTER TABLE generated_reports ADD CONSTRAINT gr_trigger_source_check
    CHECK (trigger_source IN ('SCHEDULED', 'MANUAL_RUN_NOW')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'gr_period_type_check') THEN ALTER TABLE generated_reports ADD CONSTRAINT gr_period_type_check
    CHECK (period_type IN ('DAILY', 'WEEKLY', 'MONTHLY')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'gr_generation_status_check') THEN ALTER TABLE generated_reports ADD CONSTRAINT gr_generation_status_check
    CHECK (generation_status IN ('PROCESSING', 'COMPLETED', 'FAILED')); END IF; END $$;
