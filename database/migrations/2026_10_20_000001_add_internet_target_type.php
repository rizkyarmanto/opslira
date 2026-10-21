<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;

return new class extends Migration
{
    /**
     * Adds the ping-only INTERNET target type and its ICMP_PING protocol. The
     * two CHECK constraints in 2026_01_000001 list the original sets, so the
     * new rows would be rejected without widening them first. Follows the same
     * drop-and-recreate shape as 2026_10_19_000001_update_target_credentials_auth_type_check.
     */
    public function up(): void
    {
        DB::statement('ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_target_type_check');
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_target_type_check
             CHECK (target_type IN ('NETWORK_DEVICE','WEB_APP','DOMAIN','DATABASE','MAIL_SERVER','S3_STORAGE','BESZEL_SERVER','INTERNET'))"
        );
        DB::statement('ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_protocol_type_check');
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_protocol_type_check
             CHECK (protocol_type IN ('ICMP_SNMP','HTTP_HEALTH','WHOIS_SSL','DB_NATIVE_PING','SMTP_DNSBL','S3_HEAD','BESZEL_API','ICMP_PING'))"
        );
    }

    public function down(): void
    {
        // Rows the old application cannot read or probe are dropped so the old
        // constraints can be restored; operators re-create them after a downgrade.
        DB::statement("DELETE FROM monitoring_targets WHERE target_type = 'INTERNET'");
        DB::statement('ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_target_type_check');
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_target_type_check
             CHECK (target_type IN ('NETWORK_DEVICE','WEB_APP','DOMAIN','DATABASE','MAIL_SERVER','S3_STORAGE','BESZEL_SERVER'))"
        );
        DB::statement('ALTER TABLE monitoring_targets DROP CONSTRAINT IF EXISTS mt_protocol_type_check');
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_protocol_type_check
             CHECK (protocol_type IN ('ICMP_SNMP','HTTP_HEALTH','WHOIS_SSL','DB_NATIVE_PING','SMTP_DNSBL','S3_HEAD','BESZEL_API'))"
        );
    }
};
