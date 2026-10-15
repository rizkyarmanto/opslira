<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('monitoring_daily_summaries', function (Blueprint $table) {
            $table->bigIncrements('id');
            $table->foreignUuid('target_id')->constrained('monitoring_targets')->cascadeOnDelete();
            $table->date('summary_date');
            $table->integer('total_checks')->default(0);
            $table->integer('successful_checks')->default(0);
            $table->integer('failed_checks')->default(0);
            $table->decimal('uptime_percentage', 7, 4)->default(100.0000);
            $table->decimal('avg_response_time_ms', 10, 2)->default(0);
            $table->decimal('max_response_time_ms', 10, 2)->default(0);
            $table->decimal('p95_response_time_ms', 10, 2)->default(0);
            $table->integer('total_incidents')->default(0);
            $table->integer('total_downtime_seconds')->default(0);
            $table->integer('maintenance_seconds')->default(0);
            $table->jsonb('daily_metrics_snapshot')->default('{}');
            $table->timestampTz('created_at')->useCurrent();

            $table->unique(['target_id', 'summary_date']);
        });

        DB::statement('CREATE INDEX idx_daily_summaries_date_target ON monitoring_daily_summaries (summary_date DESC, target_id)');
    }

    public function down(): void
    {
        Schema::dropIfExists('monitoring_daily_summaries');
    }
};
