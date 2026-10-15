<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('generated_reports', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->foreignUuid('schedule_id')->nullable()->constrained('report_schedules')->nullOnDelete();
            $table->string('trigger_source', 20);
            $table->string('period_type', 20);
            $table->timestampTz('period_start');
            $table->timestampTz('period_end');
            $table->string('pdf_file_path', 255)->nullable();
            $table->string('excel_file_path', 255)->nullable();
            $table->jsonb('summary_snapshot')->default('{}');
            $table->string('generation_status', 20)->default('PROCESSING');
            $table->timestampTz('created_at')->useCurrent();

            // Idempotency guard: one report per (schedule, period_start)
            $table->unique(['schedule_id', 'period_start']);
        });

        DB::statement(
            "ALTER TABLE generated_reports ADD CONSTRAINT gr_trigger_source_check
             CHECK (trigger_source IN ('SCHEDULED','MANUAL_RUN_NOW'))"
        );
        DB::statement(
            "ALTER TABLE generated_reports ADD CONSTRAINT gr_period_type_check
             CHECK (period_type IN ('DAILY','WEEKLY','MONTHLY'))"
        );
        DB::statement(
            "ALTER TABLE generated_reports ADD CONSTRAINT gr_generation_status_check
             CHECK (generation_status IN ('PROCESSING','COMPLETED','FAILED'))"
        );
    }

    public function down(): void
    {
        Schema::dropIfExists('generated_reports');
    }
};
