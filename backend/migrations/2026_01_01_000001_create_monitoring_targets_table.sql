-- Transcribed from database/migrations/2026_01_01_000001_create_monitoring_targets_table.php
CREATE TABLE IF NOT EXISTS monitoring_targets (
    id UUID PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    dashboard_group VARCHAR(30) NOT NULL,
    target_type VARCHAR(30) NOT NULL,
    environment VARCHAR(20) NOT NULL DEFAULT 'INTERNAL',
    host_or_endpoint VARCHAR(255) NOT NULL,
    port INTEGER,
    protocol_type VARCHAR(30) NOT NULL,
    config_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    check_interval_seconds INTEGER NOT NULL DEFAULT 60,
    timeout_seconds INTEGER NOT NULL DEFAULT 5,
    retry_threshold INTEGER NOT NULL DEFAULT 3,
    degraded_threshold_ms INTEGER NOT NULL DEFAULT 1000,
    current_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    is_maintenance BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_checked_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- CHECK constraints (Laravel builder cannot express these)
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_dashboard_group_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_dashboard_group_check
    CHECK (dashboard_group IN ('NETWORK', 'APP_DOMAIN_DB', 'MAIL_S3_BESZEL')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_target_type_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_target_type_check
    CHECK (target_type IN ('NETWORK_DEVICE', 'WEB_APP', 'DOMAIN', 'DATABASE', 'MAIL_SERVER', 'S3_STORAGE', 'BESZEL_SERVER')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_environment_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_environment_check
    CHECK (environment IN ('PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_protocol_type_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_protocol_type_check
    CHECK (protocol_type IN ('ICMP_SNMP', 'HTTP_HEALTH', 'WHOIS_SSL', 'DB_NATIVE_PING', 'SMTP_DNSBL', 'S3_HEAD', 'BESZEL_API')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_port_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_port_check
    CHECK (port IS NULL OR (port >= 1 AND port <= 65535)); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_check_interval_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_check_interval_check
    CHECK (check_interval_seconds >= 10); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_timeout_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_timeout_check
    CHECK (timeout_seconds >= 1 AND timeout_seconds <= 60); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_retry_threshold_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_retry_threshold_check
    CHECK (retry_threshold >= 1 AND retry_threshold <= 10); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_current_status_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_current_status_check
    CHECK (current_status IN ('UP', 'DEGRADED', 'DOWN', 'MAINTENANCE', 'PENDING')); END IF; END $$;

CREATE INDEX IF NOT EXISTS idx_targets_scheduler ON monitoring_targets (is_active, is_maintenance, next_check_at);
CREATE INDEX IF NOT EXISTS idx_targets_dashboard_env ON monitoring_targets (dashboard_group, target_type, environment);
