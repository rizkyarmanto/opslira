<?php

namespace App\Modules\Reporting\Services;

use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Carbon;

/**
 * Pure, interval-based SLA math for daily and monthly periods.
 *
 * Formulas (identical for the daily and the monthly window):
 *   Daily SLA  = (total_seconds - maintenance_seconds - downtime_seconds)
 *                / (total_seconds - maintenance_seconds) * 100
 *   Monthly SLA = (total_seconds - maintenance_seconds - downtime_seconds)
 *                 / (total_seconds - maintenance_seconds) * 100
 *   The monthly figure comes from one call over the whole month, so it is
 *   check-weighted by construction through the seconds sums and never an
 *   average of daily SLA values.
 *
 * Assumptions:
 * - Probe-interval discretization: a DOWN check owns the time until the next
 *   check, so callers pass DOWN segments (start/end pairs) instead of raw
 *   point-in-time check rows.
 * - Maintenance wins on overlap: seconds that fall inside both a maintenance
 *   window and a DOWN segment count as maintenance only; the intersection is
 *   subtracted from the raw DOWN seconds.
 * - Check truth hybrid: segments originate from check logs (check-count truth)
 *   but are weighted by wall-clock seconds rather than by raw check counts.
 *
 * Null SLA rules:
 * - has_data = false (caller found zero check rows) forces sla = null.
 * - has_data = true with (total_seconds - maintenance_seconds) <= 0 also yields
 *   sla = null; that is a fully maintained window, distinct from no data.
 *
 * Scope note: RunMidnightDataRollupCommand and the dashboard keep the current
 * check-count behavior of SlaCalculatorService. This service is the forward
 * path only and is not wired into the rollup command yet.
 *
 * Purity: no config reads, no database queries, no model writes. Every instant
 * is normalized to Asia/Jakarta at method entry; all math runs on Carbon
 * instants only.
 */
class SlaPeriodService
{
    private const TIMEZONE = 'Asia/Jakarta';

    /**
     * SLA for an arbitrary window built from maintenance and DOWN segments.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $maintenanceWindows
     * @param  array<int, array{start: Carbon, end: Carbon}>  $downIntervals
     *         DOWN segments; each segment spans from one probe check until the
     *         next check (probe-interval discretization).
     * @param  bool  $hasData
     *         Caller signals whether the window contains at least one check
     *         row. When false the result carries has_data = false and sla = null
     *         regardless of the segments passed in. Defaults to true.
     * @return array{total_seconds: int, maintenance_seconds: int, downtime_seconds: int, sla: float|null, has_data: bool}
     *         sla is a full-precision float here; callers round at their return
     *         edge. Seconds fields are whole seconds.
     */
    public function downtimeForWindow(
        MonitoringTarget $target,
        Carbon $start,
        Carbon $end,
        array $maintenanceWindows,
        array $downIntervals,
        bool $hasData = true,
    ): array {
        // $target is part of the period-builder API surface; window math is
        // pure on the given intervals and does not read target attributes.
        unset($target);

        $start = $this->toJakarta($start);
        $end = $this->toJakarta($end);

        // abs first so a reversed window never yields a negative total, round
        // because endOfDay carries .999999 microseconds (86399.999999 -> 86400).
        $totalSeconds = (int) round(abs($start->diffInSeconds($end)));

        $maintenance = $this->mergeSegments($this->clipToWindow($maintenanceWindows, $start, $end));
        $down = $this->mergeSegments($this->clipToWindow($downIntervals, $start, $end));

        $maintenanceSeconds = (int) round($this->sumSegmentSeconds($maintenance));
        $rawDownSeconds = $this->sumSegmentSeconds($down);
        // Maintenance wins on overlap: raw DOWN minus the maintenance/DOWN
        // intersection, clamped at zero so the result can never go negative.
        $downtimeSeconds = (int) round(max(0.0, $rawDownSeconds - $this->intersectionSeconds($maintenance, $down)));

        $denominator = $totalSeconds - $maintenanceSeconds;
        $sla = null;
        if ($hasData && $denominator > 0) {
            // Computed from the returned integers so the published fields
            // reproduce the published sla exactly. Full float, no rounding.
            $sla = (($denominator - $downtimeSeconds) / $denominator) * 100;
        }

        return [
            'total_seconds' => $totalSeconds,
            'maintenance_seconds' => $maintenanceSeconds,
            'downtime_seconds' => $downtimeSeconds,
            'sla' => $sla,
            'has_data' => $hasData,
        ];
    }

    /**
     * SLA for one calendar day in Asia/Jakarta.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $maintenanceWindows
     * @param  array<int, array{start: Carbon, end: Carbon}>  $downIntervals
     * @return array{date: string, sla: float|null, has_data: bool, downtime_seconds: int, maintenance_seconds: int, total_seconds: int}
     *         A day with no check rows must be requested with $hasData = false;
     *         it then returns sla = null, has_data = false and never a fake
     *         100 or 0. Rounding to 4 decimals happens only at this return edge.
     */
    public function daily(
        MonitoringTarget $target,
        Carbon $date,
        array $maintenanceWindows = [],
        array $downIntervals = [],
        bool $hasData = true,
    ): array {
        $day = $this->toJakarta($date);
        $start = $day->copy()->startOfDay();
        $end = $day->copy()->endOfDay();

        $window = $this->downtimeForWindow($target, $start, $end, $maintenanceWindows, $downIntervals, $hasData);

        return [
            'date' => $start->format('Y-m-d'),
            'sla' => $window['sla'] === null ? null : round($window['sla'], 4),
            'has_data' => $window['has_data'],
            'downtime_seconds' => $window['downtime_seconds'],
            'maintenance_seconds' => $window['maintenance_seconds'],
            'total_seconds' => $window['total_seconds'],
        ];
    }

    /**
     * SLA for the calendar month containing $anyDateInMonth, in Asia/Jakarta.
     *
     * The window comes from startOfMonth/endOfMonth so 28/29/30/31-day months
     * (including leap February) are derived from the calendar, not from
     * arithmetic on day counts. The whole month is aggregated in one core call;
     * daily SLA values are never averaged.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $maintenanceWindows
     * @param  array<int, array{start: Carbon, end: Carbon}>  $downIntervals
     * @return array{month: string, sla: float|null, has_data: bool, downtime_seconds: int, maintenance_seconds: int, total_seconds: int}
     *         A month with no check rows must be requested with $hasData = false;
     *         it then returns sla = null, has_data = false. Rounding to 4
     *         decimals happens only at this return edge.
     */
    public function monthly(
        MonitoringTarget $target,
        Carbon $anyDateInMonth,
        array $maintenanceWindows = [],
        array $downIntervals = [],
        bool $hasData = true,
    ): array {
        $anchor = $this->toJakarta($anyDateInMonth);
        $start = $anchor->copy()->startOfMonth();
        $end = $anchor->copy()->endOfMonth();

        $window = $this->downtimeForWindow($target, $start, $end, $maintenanceWindows, $downIntervals, $hasData);

        return [
            'month' => $start->format('Y-m'),
            'sla' => $window['sla'] === null ? null : round($window['sla'], 4),
            'has_data' => $window['has_data'],
            'downtime_seconds' => $window['downtime_seconds'],
            'maintenance_seconds' => $window['maintenance_seconds'],
            'total_seconds' => $window['total_seconds'],
        ];
    }

    /**
     * Allowed downtime for a target expressed as a fraction (0.999 = 99.9%).
     *
     * allowed_seconds = (1 - $target) * $periodSeconds as a float,
     * allowed_minutes = allowed_seconds / 60,
     * allowed_minutes_seconds formats the allowed budget as "Xm Ys" by rounding
     * to whole seconds first, then splitting into floor minutes plus the
     * remainder, so 43.2 seconds formats as "0m 43s" and a 59.6 second remainder
     * becomes "1m 0s" instead of "0m 60s".
     *
     * @return array{allowed_seconds: float, allowed_minutes: float, allowed_minutes_seconds: string}
     */
    public function errorBudget(float $target, int $periodSeconds): array
    {
        $allowedSeconds = (1.0 - $target) * $periodSeconds;

        return [
            'allowed_seconds' => $allowedSeconds,
            'allowed_minutes' => $allowedSeconds / 60,
            'allowed_minutes_seconds' => $this->formatDuration((int) round($allowedSeconds)),
        ];
    }

    /**
     * Compare an SLA percentage against a target and the consumed downtime
     * against the allowed error budget.
     *
     * $sla is a percentage on the 0-100 scale (as produced by this service);
     * $target is a fraction on the 0-1 scale (0.999 = 99.9%) and is compared as
     * sla >= $target * 100. A null $sla (no data or full maintenance) can never
     * be met, so met = false.
     *
     * remaining_seconds = allowedSeconds - downtimeSeconds, which may be
     * negative when the budget is exceeded; remaining_human formats it as
     * "Xm Ys" with a "-" sign prefix when negative, for example "-2m 5s"
     * or "5m 0s".
     *
     * @return array{met: bool, remaining_seconds: float, remaining_human: string}
     */
    public function status(?float $sla, float $target, float $downtimeSeconds, float $allowedSeconds): array
    {
        $remainingSeconds = $allowedSeconds - $downtimeSeconds;

        return [
            'met' => $sla !== null && $sla >= $target * 100,
            'remaining_seconds' => $remainingSeconds,
            'remaining_human' => $this->formatDuration((int) round($remainingSeconds)),
        ];
    }

    /**
     * Classify a single monitoring log row as DOWN for SLA purposes.
     *
     * True when the log says DOWN, when the HTTP status is a 5xx, or when the
     * measured response time exceeds the target's degraded_threshold_ms
     * (integer cast on the model).
     *
     * Timeout mapping: a probe timeout is persisted as a DOWN log whose
     * error_message mentions the timeout, so the status branch below already
     * classifies timeouts as down.
     */
    public function isDown(array $log, MonitoringTarget $target): bool
    {
        if (($log['status'] ?? null) === 'DOWN') {
            return true;
        }

        if ((int) ($log['http_status_code'] ?? 0) >= 500) {
            return true;
        }

        return isset($log['response_time_ms'])
            && $log['response_time_ms'] > $target->degraded_threshold_ms;
    }

    /**
     * Copy an instant into the Asia/Jakarta timezone used by every period
     * boundary in this service.
     */
    private function toJakarta(Carbon $instant): Carbon
    {
        return $instant->copy()->setTimezone(self::TIMEZONE);
    }

    /**
     * Clip every interval to [$start, $end] (max of starts, min of ends) and
     * drop empty intervals where end <= start.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $intervals
     * @return array<int, array{start: Carbon, end: Carbon}>
     */
    private function clipToWindow(array $intervals, Carbon $start, Carbon $end): array
    {
        $clipped = [];

        foreach ($intervals as $interval) {
            $intervalStart = $this->toJakarta($interval['start']);
            $intervalEnd = $this->toJakarta($interval['end']);

            if ($intervalStart->lt($start)) {
                $intervalStart = $start->copy();
            }
            if ($intervalEnd->gt($end)) {
                $intervalEnd = $end->copy();
            }

            if ($intervalEnd->lte($intervalStart)) {
                continue;
            }

            $clipped[] = ['start' => $intervalStart, 'end' => $intervalEnd];
        }

        return $clipped;
    }

    /**
     * Sort-then-merge overlapping or adjacent segments inside one list.
     *
     * The caller clips probe cadence to segment ends, so consecutive segments
     * can overlap or meet exactly (end of one equals start of the next);
     * merging keeps each list from double-counting those seconds before the
     * maintenance/down intersection is computed.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $segments
     * @return array<int, array{start: Carbon, end: Carbon}>
     */
    private function mergeSegments(array $segments): array
    {
        if ($segments === []) {
            return [];
        }

        usort($segments, fn (array $a, array $b): int => ($a['start']->lt($b['start']) ? -1 : ($a['start']->gt($b['start']) ? 1 : 0)));

        $merged = [];
        $current = $segments[0];

        for ($i = 1, $count = count($segments); $i < $count; $i++) {
            $next = $segments[$i];

            if ($next['start']->lte($current['end'])) {
                // Overlapping or adjacent: extend only when the next segment
                // reaches further, otherwise the current one already covers it.
                if ($next['end']->gt($current['end'])) {
                    $current['end'] = $next['end'];
                }
                continue;
            }

            $merged[] = $current;
            $current = $next;
        }

        $merged[] = $current;

        return $merged;
    }

    /**
     * Sum of all segment lengths in seconds.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $segments
     */
    private function sumSegmentSeconds(array $segments): float
    {
        $total = 0.0;

        foreach ($segments as $segment) {
            $total += abs($segment['start']->diffInSeconds($segment['end']));
        }

        return $total;
    }

    /**
     * Total seconds covered by both lists. Both lists must already be sorted
     * and merged; a two-pointer sweep over the disjoint segments is enough.
     *
     * @param  array<int, array{start: Carbon, end: Carbon}>  $a
     * @param  array<int, array{start: Carbon, end: Carbon}>  $b
     */
    private function intersectionSeconds(array $a, array $b): float
    {
        $seconds = 0.0;
        $i = 0;
        $j = 0;
        $countA = count($a);
        $countB = count($b);

        while ($i < $countA && $j < $countB) {
            $overlapStart = $a[$i]['start']->gt($b[$j]['start']) ? $a[$i]['start'] : $b[$j]['start'];
            $overlapEnd = $a[$i]['end']->lt($b[$j]['end']) ? $a[$i]['end'] : $b[$j]['end'];

            if ($overlapEnd->gt($overlapStart)) {
                $seconds += abs($overlapStart->diffInSeconds($overlapEnd));
            }

            if ($a[$i]['end']->lt($b[$j]['end'])) {
                $i++;
            } else {
                $j++;
            }
        }

        return $seconds;
    }

    /**
     * Format whole seconds as "Xm Ys", with a "-" prefix for negatives
     * (for example "0m 43s", "5m 0s", "-2m 5s").
     */
    private function formatDuration(int $totalSeconds): string
    {
        $sign = $totalSeconds < 0 ? '-' : '';
        $absolute = abs($totalSeconds);

        return sprintf('%s%dm %ds', $sign, intdiv($absolute, 60), $absolute % 60);
    }
}
