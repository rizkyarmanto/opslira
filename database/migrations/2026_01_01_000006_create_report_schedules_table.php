<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('report_schedules', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->string('name', 150);
            $table->string('period_type', 20);
            $table->string('cron_expression', 60);
            $table->string('timezone', 50)->default('Asia/Jakarta');
            $table->jsonb('scope_filter')->default('{"dashboard_groups": ["ALL"], "environments": ["ALL"]}');
            $table->jsonb('report_formats')->default('["HTML", "PDF", "XLSX"]');
            $table->jsonb('recipients_to')->default('[]');
            $table->jsonb('recipients_cc')->default('[]');
            $table->boolean('send_only_on_issues')->default(false);
            $table->boolean('is_active')->default(true);
            $table->timestampTz('last_run_at')->nullable();
            $table->timestampTz('next_run_at');
            $table->foreignUuid('created_by')->nullable()->constrained('users')->nullOnDelete();
            $table->timestampsTz();
        });

        DB::statement(
            "ALTER TABLE report_schedules ADD CONSTRAINT rs_period_type_check
             CHECK (period_type IN ('DAILY','WEEKLY','MONTHLY','CUSTOM_CRON'))"
        );

        DB::statement('CREATE INDEX idx_report_schedules_next_run ON report_schedules (is_active, next_run_at)');
    }

    public function down(): void
    {
        Schema::dropIfExists('report_schedules');
    }
};
