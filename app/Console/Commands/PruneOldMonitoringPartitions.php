<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;

/**
 * Prunes monitoring_logs partitions older than 90 days and pre-creates the
 * next 2 months of partitions.
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:131-133
 */
class PruneOldMonitoringPartitions extends Command
{
    protected $signature = 'monitoring:prune-partitions {--retention-days=90}';

    protected $description = 'Drop monitoring_logs partitions older than retention, create upcoming ones';

    public function handle(): int
    {
        $retentionDays = (int) $this->option('retention-days');
        $cutoff = now()->subDays($retentionDays);
        $dropped = 0;
        $created = 0;

        // List existing partitions
        $partitions = DB::select(
            "SELECT inhrelid::regclass::text AS name
             FROM pg_inherits
             WHERE inhparent = 'monitoring_logs'::regclass"
        );

        foreach ($partitions as $partition) {
            $name = $partition->name;

            if (preg_match('/monitoring_logs_(\d{4})_(\d{2})/', $name, $m)) {
                $partitionStart = Carbon::createFromDate((int) $m[1], (int) $m[2], 1)->startOfMonth();

                if ($partitionStart->lt($cutoff->copy()->startOfMonth())) {
                    DB::statement("DROP TABLE IF EXISTS {$name}");
                    $this->warn("Dropped partition {$name}");
                    $dropped++;
                }
            }
        }

        // Pre-create next 2 months
        foreach ([0, 1, 2] as $offset) {
            if ($this->createPartition(now()->addMonthsNoOverflow($offset))) {
                $created++;
            }
        }

        $this->info("Prune complete: {$dropped} dropped, {$created} ensured.");

        return self::SUCCESS;
    }

    private function createPartition(Carbon $date): bool
    {
        $suffix = $date->format('Y_m');
        $start = $date->copy()->startOfMonth()->format('Y-m-d H:i:sP');
        $end = $date->copy()->addMonthNoOverflow()->startOfMonth()->format('Y-m-d H:i:sP');

        $exists = DB::selectOne("SELECT to_regclass('monitoring_logs_{$suffix}') AS tbl");
        if ($exists->tbl !== null) {
            return false;
        }

        DB::statement(
            "CREATE TABLE IF NOT EXISTS monitoring_logs_{$suffix} PARTITION OF monitoring_logs
             FOR VALUES FROM ('{$start}') TO ('{$end}')"
        );

        return true;
    }
}
