-- Transcribed from database/migrations/2026_01_01_000008_create_email_delivery_logs_table.php
CREATE TABLE IF NOT EXISTS email_delivery_logs (
    id UUID PRIMARY KEY,
    email_category VARCHAR(30) NOT NULL,
    generated_report_id UUID REFERENCES generated_reports (id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents (id) ON DELETE CASCADE,
    recipient_email VARCHAR(150) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    delivery_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    retry_count INTEGER NOT NULL DEFAULT 0,
    smtp_response_or_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'edl_category_check') THEN ALTER TABLE email_delivery_logs ADD CONSTRAINT edl_category_check
    CHECK (email_category IN ('SCHEDULED_REPORT', 'INCIDENT_ALERT', 'EXPIRY_WARNING')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'edl_status_check') THEN ALTER TABLE email_delivery_logs ADD CONSTRAINT edl_status_check
    CHECK (delivery_status IN ('PENDING', 'SENT', 'RETRYING', 'FAILED')); END IF; END $$;

CREATE INDEX IF NOT EXISTS idx_email_logs_status_created ON email_delivery_logs (delivery_status, created_at DESC);
