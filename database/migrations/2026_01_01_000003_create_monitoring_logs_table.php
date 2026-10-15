<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;

return new class extends Migration
{
    public function up(): void
    {
        // Declarative range partitioning by checked_at (monthly)
        DB::statement(<<<'SQL'
            CREATE TABLE monitoring_logs (
                id BIGSERIAL,
                target_id UUID NOT NULL,
                status VARCHAR(20) NOT NULL CHECK (status IN ('UP','DEGRADED','DOWN')),
                response_time_ms NUMERIC(10, 2) NULL,
                http_status_code INTEGER NULL,
                error_message TEXT NULL,
                metrics_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
                checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                PRIMARY KEY (id, checked_at)
            ) PARTITION BY RANGE (checked_at)
        SQL);

        DB::statement('CREATE INDEX idx_monitoring_logs_target_time ON monitoring_logs (target_id, checked_at DESC)');

        // Create current month + next 2 months partitions
        $this->createPartition(now());
        $this->createPartition(now()->addMonthNoOverflow());
        $this->createPartition(now()->addMonthsNoOverflow(2));
    }

    private function createPartition(Carbon $date): void
    {
        $suffix = $date->format('Y_m');
        $start = $date->copy()->startOfMonth()->format('Y-m-d H:i:sP');
        $end = $date->copy()->addMonthNoOverflow()->startOfMonth()->format('Y-m-d H:i:sP');

        DB::statement(
            "CREATE TABLE IF NOT EXISTS monitoring_logs_{$suffix} PARTITION OF monitoring_logs
             FOR VALUES FROM ('{$start}') TO ('{$end}')"
        );
    }

    public function down(): void
    {
        DB::statement('DROP TABLE IF EXISTS monitoring_logs CASCADE');
    }
};
