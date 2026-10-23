<?php

namespace App\Modules\Reporting\Services;

use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;

/**
 * Bridge from raw probe logs to SlaPeriodService math for dashboard display.
 *
 * Builds DOWN segments from monitoring_logs (status/http/latency classified
 * via SlaPeriodService::isDown) and aggregates per-target and pooled
 * cross-target SLA in wall-clock seconds for one window.
 */
class SlaDashboardService
{
    public function __construct(private readonly SlaPeriodService $periods)
    {
    }

    /**
     * SLA for one window across active targets (or a subset).
     *
     * DOWN segment rule: each DOWN-classified log owns time from its
     * checked_at until the next log's checked_at for the same target, or
     * $end for the last log (probe-interval discretization).
     *
     * @param  array<int, string>|null  $targetIds
     * @param  array<string, array<int, array{start: Carbon, end: Carbon}>>  $maintenanceByTarget
     * @return array{
     *   sla: float|null,
     *   has_data: bool,
     *   downtime_seconds: int,
     *   maintenance_seconds: int,
     *   total_seconds: int,
     *   targets: array<int, array{target_id: string, sla: float|null, has_data: bool, downtime_seconds: int, maintenance_seconds: int, total_seconds: int}>
     * }
     */
    public function forWindow(
        Carbon $start,
        Carbon $end,
        ?array $targetIds = null,
        array $maintenanceByTarget = [],
    ): array {
        $start = $start->copy()->setTimezone('Asia/Jakarta');
        $end = $end->copy()->setTimezone('Asia/Jakarta');

        $query = MonitoringTarget::where('is_active', true);
        if ($targetIds !== null) {
            $query->whereIn('id', $targetIds);
        }
        $targets = $query->get();

        $totalSeconds = 0;
        $maintenanceSeconds = 0;
        $downtimeSeconds = 0;
        $hasData = false;
        $rows = [];

        foreach ($targets as $target) {
            $logs = DB::table('monitoring_logs')
                ->where('target_id', $target->id)
                ->whereBetween('checked_at', [$start, $end])
                ->orderBy('checked_at')
                ->get(['status', 'http_status_code', 'response_time_ms', 'checked_at']);

            $segments = $this->buildDownSegments($target, $logs, $end);
            $targetHasData = $logs->isNotEmpty();

            // Maintenance empty for now; manual schedule input fills
            // $maintenanceByTarget[$id] later without changing this service.
            $result = $this->periods->downtimeForWindow(
                $target,
                $start,
                $end,
                $maintenanceByTarget[$target->id] ?? [],
                $segments,
                $targetHasData,
            );

            if ($targetHasData) {
                $hasData = true;
                $totalSeconds += $result['total_seconds'];
                $maintenanceSeconds += $result['maintenance_seconds'];
                $downtimeSeconds += $result['downtime_seconds'];
            }

            $rows[] = [
                'target_id' => $target->id,
                'sla' => $result['sla'],
                'has_data' => $result['has_data'],
                'downtime_seconds' => $result['downtime_seconds'],
                'maintenance_seconds' => $result['maintenance_seconds'],
                'total_seconds' => $result['total_seconds'],
            ];
        }

        $denominator = $totalSeconds - $maintenanceSeconds;
        $sla = null;
        if ($hasData && $denominator > 0) {
            $sla = (($denominator - $downtimeSeconds) / $denominator) * 100;
        }

        return [
            'sla' => $sla,
            'has_data' => $hasData,
            'downtime_seconds' => $downtimeSeconds,
            'maintenance_seconds' => $maintenanceSeconds,
            'total_seconds' => $totalSeconds,
            'targets' => $rows,
        ];
    }

    /**
     * @param  \Illuminate\Support\Collection<int, object>  $logs
     * @return array<int, array{start: Carbon, end: Carbon}>
     */
    private function buildDownSegments(MonitoringTarget $target, $logs, Carbon $end): array
    {
        $segments = [];
        $count = $logs->count();

        for ($i = 0; $i < $count; $i++) {
            $log = $logs[$i];
            $row = [
                'status' => $log->status,
                'http_status_code' => $log->http_status_code,
                'response_time_ms' => $log->response_time_ms === null ? null : (float) $log->response_time_ms,
            ];

            if (! $this->periods->isDown($row, $target)) {
                continue;
            }

            $segmentStart = Carbon::parse($log->checked_at)->setTimezone('Asia/Jakarta');
            $segmentEnd = $i + 1 < $count
                ? Carbon::parse($logs[$i + 1]->checked_at)->setTimezone('Asia/Jakarta')
                : $end->copy();

            if ($segmentEnd->gt($segmentStart)) {
                $segments[] = ['start' => $segmentStart, 'end' => $segmentEnd];
            }
        }

        return $segments;
    }
}
