<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;

return new class extends Migration
{
    /**
     * The credential matrix gained HTTP_BASIC and MAIL_AUTH, and the Beszel type
     * dropped its _TOKEN suffix (BESZEL covers both login and API token now).
     * The table CHECK constraint in 2026_01_01_000002 listed the old set, which
     * rejected the new rows.
     */
    public function up(): void
    {
        DB::statement('ALTER TABLE target_credentials DROP CONSTRAINT IF EXISTS tc_auth_type_check');
        DB::statement(
            "ALTER TABLE target_credentials ADD CONSTRAINT tc_auth_type_check
             CHECK (auth_type IN ('NONE','DB_AUTH','SNMP_V2C','SNMP_V3','S3_KEY','BESZEL','HTTP_BASIC','HTTP_BEARER','MAIL_AUTH'))"
        );
    }

    public function down(): void
    {
        // Rows using auth types the old application cannot read or probe are
        // dropped so the old constraint can be restored; operators re-enter
        // credentials through the form after a downgrade.
        DB::statement(
            "DELETE FROM target_credentials
             WHERE auth_type IN ('BESZEL','HTTP_BASIC','MAIL_AUTH')"
        );
        DB::statement('ALTER TABLE target_credentials DROP CONSTRAINT IF EXISTS tc_auth_type_check');
        DB::statement(
            "ALTER TABLE target_credentials ADD CONSTRAINT tc_auth_type_check
             CHECK (auth_type IN ('NONE','DB_AUTH','SNMP_V2C','SNMP_V3','S3_KEY','BESZEL_TOKEN','HTTP_BEARER'))"
        );
    }
};
