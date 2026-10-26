-- Transcribed from database/migrations/2026_01_01_000002_create_target_credentials_table.php
CREATE TABLE IF NOT EXISTS target_credentials (
    id UUID PRIMARY KEY,
    target_id UUID NOT NULL UNIQUE REFERENCES monitoring_targets (id) ON DELETE CASCADE,
    auth_type VARCHAR(30) NOT NULL,
    encrypted_payload TEXT NOT NULL,
    encryption_iv VARCHAR(64) NOT NULL,
    encryption_tag VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tc_auth_type_check') THEN ALTER TABLE target_credentials ADD CONSTRAINT tc_auth_type_check
    CHECK (auth_type IN ('NONE', 'DB_AUTH', 'SNMP_V2C', 'SNMP_V3', 'S3_KEY', 'BESZEL_TOKEN', 'HTTP_BEARER')); END IF; END $$;
