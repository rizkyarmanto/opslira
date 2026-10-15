<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('monitoring_targets', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->string('name', 150);
            $table->string('dashboard_group', 30);
            $table->string('target_type', 30);
            $table->string('environment', 20)->default('INTERNAL');
            $table->string('host_or_endpoint', 255);
            $table->integer('port')->nullable();
            $table->string('protocol_type', 30);
            $table->jsonb('config_payload')->default('{}');
            $table->integer('check_interval_seconds')->default(60);
            $table->integer('timeout_seconds')->default(5);
            $table->integer('retry_threshold')->default(3);
            $table->integer('degraded_threshold_ms')->default(1000);
            $table->string('current_status', 20)->default('PENDING');
            $table->boolean('is_maintenance')->default(false);
            $table->boolean('is_active')->default(true);
            $table->timestampTz('last_checked_at')->nullable();
            $table->timestampTz('next_check_at')->useCurrent();
            $table->foreignUuid('created_by')->nullable()->constrained('users')->nullOnDelete();
            $table->timestampsTz();
        });

        // CHECK constraints (Laravel builder cannot express these)
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_dashboard_group_check
             CHECK (dashboard_group IN ('NETWORK', 'APP_DOMAIN_DB', 'MAIL_S3_BESZEL'))"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_target_type_check
             CHECK (target_type IN ('NETWORK_DEVICE','WEB_APP','DOMAIN','DATABASE','MAIL_SERVER','S3_STORAGE','BESZEL_SERVER'))"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_environment_check
             CHECK (environment IN ('PRODUCTION','STAGING','DEVELOPMENT','INTERNAL'))"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_protocol_type_check
             CHECK (protocol_type IN ('ICMP_SNMP','HTTP_HEALTH','WHOIS_SSL','DB_NATIVE_PING','SMTP_DNSBL','S3_HEAD','BESZEL_API'))"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_port_check
             CHECK (port IS NULL OR (port >= 1 AND port <= 65535))"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_check_interval_check
             CHECK (check_interval_seconds >= 10)"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_timeout_check
             CHECK (timeout_seconds >= 1 AND timeout_seconds <= 60)"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_retry_threshold_check
             CHECK (retry_threshold >= 1 AND retry_threshold <= 10)"
        );
        DB::statement(
            "ALTER TABLE monitoring_targets ADD CONSTRAINT mt_current_status_check
             CHECK (current_status IN ('UP','DEGRADED','DOWN','MAINTENANCE','PENDING'))"
        );

        DB::statement('CREATE INDEX idx_targets_scheduler ON monitoring_targets (is_active, is_maintenance, next_check_at)');
        DB::statement('CREATE INDEX idx_targets_dashboard_env ON monitoring_targets (dashboard_group, target_type, environment)');
    }

    public function down(): void
    {
        Schema::dropIfExists('monitoring_targets');
    }
};
