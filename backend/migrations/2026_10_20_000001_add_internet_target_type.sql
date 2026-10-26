-- Transcribed from database/migrations/2026_10_20_000001_add_internet_target_type.php
-- Adds the ping-only INTERNET target type and its ICMP_PING protocol; widens
-- the two CHECK constraints created in 2026_01_01_000001.
ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_target_type_check;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_target_type_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_target_type_check
    CHECK (target_type IN ('NETWORK_DEVICE', 'WEB_APP', 'DOMAIN', 'DATABASE', 'MAIL_SERVER', 'S3_STORAGE', 'BESZEL_SERVER', 'INTERNET')); END IF; END $$;

ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_protocol_type_check;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'mt_protocol_type_check') THEN ALTER TABLE monitoring_targets ADD CONSTRAINT mt_protocol_type_check
    CHECK (protocol_type IN ('ICMP_SNMP', 'HTTP_HEALTH', 'WHOIS_SSL', 'DB_NATIVE_PING', 'SMTP_DNSBL', 'S3_HEAD', 'BESZEL_API', 'ICMP_PING')); END IF; END $$;
