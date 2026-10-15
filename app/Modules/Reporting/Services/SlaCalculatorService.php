<?php

namespace App\Modules\Reporting\Services;

use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;

/**
 * SLA / MTTR / MTBF calculations.
 * docs/gemini/product_requirements_document_prd.md:114-129
 */
class SlaCalculatorService
{
    /**
     * @return array{
     *   uptime_percentage: float,
     *   mttr_seconds: float,
     *   mtbf_seconds: float,
     *   total_downtime_seconds: int,
     *   incident_count: int,
     *   total_checks: int
     * }
     */
    public function calculate(MonitoringTarget $target, Carbon $start, Carbon $end): array
    {
        $totalSeconds = $start->diffInSeconds($end);

        $logs = DB::table('monitoring_logs')
            ->where('target_id', $target->id)
            ->whereBetween('checked_at', [$start, $end])
            ->selectRaw("
                COUNT(*) as total_checks,
                COUNT(*) FILTER (WHERE status = 'DOWN') as down_checks
            ")
            ->first();

        $totalChecks = (int) ($logs->total_checks ?? 0);
        $downChecks = (int) ($logs->down_checks ?? 0);

        $incidents = DB::table('incidents')
            ->where('target_id', $target->id)
            ->whereBetween('started_at', [$start, $end])
            ->get();

        $incidentCount = $incidents->count();
        $downtimeSeconds = (int) $incidents->sum(fn ($i) => $i->duration_seconds ?? 0);

        // Maintenance time (target paused)
        $maintenanceSeconds = 0;

        $uptime = $totalChecks > 0
            ? (($totalChecks - $downChecks) / $totalChecks) * 100
            : 100.0;

        // MTTR = total downtime / number of incidents
        $mttr = $incidentCount > 0 ? $downtimeSeconds / $incidentCount : 0.0;

        // MTBF = (total uptime seconds) / number of incidents
        $uptimeSeconds = max(0, $totalSeconds - $maintenanceSeconds - $downtimeSeconds);
        $mtbf = $incidentCount > 0 ? $uptimeSeconds / $incidentCount : (float) $uptimeSeconds;

        return [
            'uptime_percentage' => round($uptime, 4),
            'mttr_seconds' => round($mttr, 2),
            'mtbf_seconds' => round($mtbf, 2),
            'total_downtime_seconds' => $downtimeSeconds,
            'incident_count' => $incidentCount,
            'total_checks' => $totalChecks,
        ];
    }
}
