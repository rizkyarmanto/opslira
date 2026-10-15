<?php

namespace App\Modules\Reporting\Jobs;

use App\Modules\Reporting\Builders\ExcelReportBuilder;
use App\Modules\Reporting\Builders\PdfReportBuilder;
use App\Modules\Reporting\Models\EmailDeliveryLog;
use App\Modules\Reporting\Models\GeneratedReport;
use App\Modules\Reporting\Models\ReportSchedule;
use App\Modules\Reporting\Services\SlaCalculatorService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\Mail;

/**
 * Generates daily/weekly/monthly reports (PDF + Excel) and emails recipients.
 * Runs on queue:reports with backoff 60/300/900 and max 3 tries.
 */
class GenerateAndSendReportJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public int $tries = 3;

    public int $timeout = 120;

    public function __construct(public string $scheduleId)
    {
        $this->onQueue('reports');
    }

    /**
     * @return array<int, int>
     */
    public function backoff(): array
    {
        return [60, 300, 900];
    }

    /**
     * Generate one report for the schedule's current period.
     *
     * Returns a short status so a manual "run now" can report what actually
     * happened instead of always claiming success (R-26, R-35):
     * - `skipped`  a report for this period already exists, nothing was redone
     * - `failed`   generation raised, the report row is marked FAILED
     * - `created`  a new report was generated
     *
     * @return array{outcome: string, message: string, report_id?: string}
     */
    public function handle(
        SlaCalculatorService $sla,
        PdfReportBuilder $pdf,
        ExcelReportBuilder $excel,
    ): array {
        $schedule = ReportSchedule::find($this->scheduleId);

        if (! $schedule) {
            return ['outcome' => 'failed', 'message' => 'Report schedule not found.'];
        }

        [$periodStart, $periodEnd] = $this->resolvePeriod($schedule->period_type);

        // Idempotency: one report per schedule per period, but a failed attempt stays
        // retryable, otherwise a single failure would block the period forever.
        $existing = GeneratedReport::where('schedule_id', $schedule->id)
            ->where('period_start', $periodStart)
            ->where('generation_status', 'COMPLETED')
            ->first();

        if ($existing) {
            return [
                'outcome' => 'skipped',
                'message' => 'A report for this period already exists; it was not regenerated.',
                'report_id' => $existing->id,
            ];
        }

        // Reuse the failed row for this period rather than piling up duplicates.
        $report = GeneratedReport::where('schedule_id', $schedule->id)
            ->where('period_start', $periodStart)
            ->where('generation_status', '!=', 'COMPLETED')
            ->first();

        if ($report) {
            $report->update(['generation_status' => 'PROCESSING', 'trigger_source' => 'MANUAL_RUN_NOW']);
        } else {
            $report = GeneratedReport::create([
                'schedule_id' => $schedule->id,
                'trigger_source' => 'SCHEDULED',
                'period_type' => $schedule->period_type === 'CUSTOM_CRON' ? 'DAILY' : $schedule->period_type,
                'period_start' => $periodStart,
                'period_end' => $periodEnd,
                'generation_status' => 'PROCESSING',
            ]);
        }

        try {
            $data = $this->buildReportData($schedule, $sla, $periodStart, $periodEnd);

            // Nothing to report and the schedule only sends on issues
            if ($schedule->send_only_on_issues && $data['summary']['total_incidents'] === 0) {
                $report->update(['generation_status' => 'COMPLETED', 'summary_snapshot' => $data['summary']]);

                return [
                    'outcome' => 'skipped',
                    'message' => 'No incidents in this period; the report was not sent.',
                    'report_id' => $report->id,
                ];
            }

            $template = 'reports.'.strtolower($data['period_type']);

            if (in_array('PDF', $schedule->report_formats, true)) {
                $report->update(['pdf_file_path' => $pdf->build($data, $template)]);
            }

            if (in_array('XLSX', $schedule->report_formats, true)) {
                $report->update(['excel_file_path' => $excel->build($data)]);
            }

            $report->update([
                'generation_status' => 'COMPLETED',
                'summary_snapshot' => $data['summary'],
            ]);

            $this->sendEmails($schedule, $report, $data);

            return [
                'outcome' => 'created',
                'message' => 'Report created.',
                'report_id' => $report->id,
            ];
        } catch (\Throwable $e) {
            $report->update(['generation_status' => 'FAILED']);

            throw $e;
        }
    }

    /**
     * @return array{0: Carbon, 1: Carbon}
     */
    private function resolvePeriod(string $periodType): array
    {
        $end = now();

        $start = match ($periodType) {
            'WEEKLY' => now()->subWeek(),
            'MONTHLY' => now()->subMonthNoOverflow(),
            default => now()->subDay(),
        };

        return [$start->startOfDay(), $end];
    }

    /**
     * @return array<string, mixed>
     */
    private function buildReportData(
        ReportSchedule $schedule,
        SlaCalculatorService $sla,
        Carbon $start,
        Carbon $end,
    ): array {
        $scope = $schedule->scope_filter ?? [];
        $groups = $scope['dashboard_groups'] ?? ['ALL'];
        $environments = $scope['environments'] ?? ['ALL'];

        $query = MonitoringTarget::where('is_active', true);
        if (! in_array('ALL', $groups, true)) {
            $query->whereIn('dashboard_group', $groups);
        }
        if (! in_array('ALL', $environments, true)) {
            $query->whereIn('environment', $environments);
        }

        $targets = $query->get();

        $rows = [];
        $totalDowntime = 0;
        $totalIncidents = 0;
        $slaSum = 0;

        foreach ($targets as $target) {
            $result = $sla->calculate($target, $start, $end);

            $rows[] = [
                'name' => $target->name,
                'dashboard_group' => $target->dashboard_group,
                'environment' => $target->environment,
                'target_type' => $target->target_type,
                ...$result,
            ];

            $totalDowntime += $result['total_downtime_seconds'];
            $totalIncidents += $result['incident_count'];
            $slaSum += $result['uptime_percentage'];
        }

        $globalSla = count($rows) > 0 ? round($slaSum / count($rows), 2) : 100.0;

        return [
            'period_type' => $schedule->period_type === 'CUSTOM_CRON' ? 'DAILY' : $schedule->period_type,
            'period_label' => $start->format('d M Y').' to '.$end->format('d M Y'),
            'generated_at' => now()->timezone('Asia/Jakarta')->toDateTimeString().' WIB',
            'schedule_name' => $schedule->name,
            'rows' => $rows,
            'summary' => [
                'global_sla' => $globalSla,
                'total_incidents' => $totalIncidents,
                'total_downtime_seconds' => $totalDowntime,
                'mttr_seconds' => $totalIncidents > 0 ? round($totalDowntime / $totalIncidents, 2) : 0,
                'total_targets' => count($rows),
            ],
        ];
    }

    /**
     * @param  array<string, mixed>  $data
     */
    private function sendEmails(ReportSchedule $schedule, GeneratedReport $report, array $data): void
    {
        $recipients = array_merge($schedule->recipients_to ?? [], $schedule->recipients_cc ?? []);

        foreach ($recipients as $email) {
            $log = EmailDeliveryLog::create([
                'email_category' => 'SCHEDULED_REPORT',
                'generated_report_id' => $report->id,
                'recipient_email' => $email,
                'subject' => "[Opslira] {$data['period_type']} report, {$schedule->name}",
                'delivery_status' => 'PENDING',
            ]);

            try {
                Mail::raw(
                    "Report {$data['period_label']}\nGlobal SLA: {$data['summary']['global_sla']}%\nIncidents: {$data['summary']['total_incidents']}",
                    function ($message) use ($email, $schedule, $data) {
                        $message->to($email)->subject("[Opslira] {$data['period_type']} report, {$schedule->name}");
                    },
                );

                $log->update(['delivery_status' => 'SENT', 'sent_at' => now()]);
            } catch (\Throwable $e) {
                $log->update([
                    'delivery_status' => 'FAILED',
                    'retry_count' => 1,
                    'smtp_response_or_error' => $e->getMessage(),
                ]);
            }
        }
    }
}
