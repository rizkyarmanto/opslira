-- Transcribed from database/migrations/2026_01_01_000005_create_incidents_table.php
CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY,
    target_id UUID NOT NULL REFERENCES monitoring_targets (id) ON DELETE CASCADE,
    severity VARCHAR(20) NOT NULL DEFAULT 'CRITICAL',
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    root_cause_message TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'incidents_severity_check') THEN ALTER TABLE incidents ADD CONSTRAINT incidents_severity_check
    CHECK (severity IN ('CRITICAL', 'WARNING')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'incidents_status_check') THEN ALTER TABLE incidents ADD CONSTRAINT incidents_status_check
    CHECK (status IN ('OPEN', 'ACKNOWLEDGED', 'RESOLVED')); END IF; END $$;

CREATE INDEX IF NOT EXISTS idx_incidents_target_started ON incidents (target_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_open ON incidents (status) WHERE status <> 'RESOLVED';
