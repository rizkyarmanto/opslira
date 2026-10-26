-- Transcribed from database/migrations/2026_10_19_000001_update_target_credentials_auth_type_check.php
-- The credential matrix gained HTTP_BASIC and MAIL_AUTH, and the Beszel type
-- dropped its _TOKEN suffix (BESZEL covers both login and API token now).
ALTER TABLE target_credentials DROP CONSTRAINT IF EXISTS tc_auth_type_check;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tc_auth_type_check') THEN ALTER TABLE target_credentials ADD CONSTRAINT tc_auth_type_check
    CHECK (auth_type IN ('NONE', 'DB_AUTH', 'SNMP_V2C', 'SNMP_V3', 'S3_KEY', 'BESZEL', 'HTTP_BASIC', 'HTTP_BEARER', 'MAIL_AUTH')); END IF; END $$;
