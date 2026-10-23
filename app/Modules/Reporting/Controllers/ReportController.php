<?php

namespace App\Modules\Reporting\Controllers;

use App\Modules\Reporting\Jobs\GenerateAndSendReportJob;
use App\Modules\Reporting\Models\EmailDeliveryLog;
use App\Modules\Reporting\Models\GeneratedReport;
use App\Modules\Reporting\Models\ReportSchedule;
use App\Modules\Reporting\Services\CronNextRunCalculator;
use Cron\CronExpression;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class ReportController
{
    public function __construct(private readonly CronNextRunCalculator $calculator)
    {
    }

    public function schedules(): Response
    {
        return Inertia::render('Reports/ScheduleManager', [
            'schedules' => ReportSchedule::orderBy('name')->get(),
        ]);
    }

    public function emailLogs(): Response
    {
        return Inertia::render('Reports/EmailDeliveryLogs', [
            'logs' => EmailDeliveryLog::orderByDesc('created_at')->limit(200)->get(),
            'reports' => GeneratedReport::orderByDesc('created_at')->limit(50)->get(),
        ]);
    }

    public function store(Request $request): JsonResponse
    {
        $data = $request->validate([
            'name' => ['required', 'string', 'max:150'],
            'period_type' => ['required', 'in:DAILY,WEEKLY,MONTHLY,CUSTOM_CRON'],
            'cron_expression' => $this->cronRules(),
            'timezone' => ['string', 'max:50'],
            'recipients_to' => ['array'],
            'recipients_to.*' => ['email'],
            'recipients_cc' => ['array'],
            'recipients_cc.*' => ['email'],
            'send_only_on_issues' => ['boolean'],
            'report_formats' => ['required', 'array', 'min:1'],
            'report_formats.*' => ['in:PDF,XLSX,HTML'],
        ]);

        $schedule = ReportSchedule::create([
            ...$data,
            'next_run_at' => $this->calculator->next(
                (string) $data['cron_expression'],
                $data['timezone'] ?? 'Asia/Jakarta',
            ),
            'is_active' => true,
        ]);

        return response()->json(['schedule' => $schedule], 201);
    }

    public function update(Request $request, ReportSchedule $schedule): JsonResponse
    {
        // Partial payloads are normal here: the card toggle sends only is_active,
        // so every rule is `sometimes` and a full edit still validates cleanly.
        $request->validate([
            'name' => ['sometimes', 'required', 'string', 'max:150'],
            'period_type' => ['sometimes', 'in:DAILY,WEEKLY,MONTHLY,CUSTOM_CRON'],
            'cron_expression' => ['sometimes', ...$this->cronRules()],
            'timezone' => ['sometimes', 'string', 'max:50'],
            'recipients_to' => ['sometimes', 'array'],
            'recipients_to.*' => ['email'],
            'recipients_cc' => ['sometimes', 'array'],
            'recipients_cc.*' => ['email'],
            'send_only_on_issues' => ['sometimes', 'boolean'],
            'report_formats' => ['sometimes', 'array', 'min:1'],
            'report_formats.*' => ['in:PDF,XLSX,HTML'],
            'is_active' => ['sometimes', 'boolean'],
        ]);

        $previousCron = $schedule->cron_expression;
        $previousTimezone = $schedule->timezone;

        $schedule->update($request->only([
            'name', 'period_type', 'cron_expression', 'timezone', 'recipients_to',
            'recipients_cc', 'send_only_on_issues', 'report_formats', 'is_active',
        ]));

        // The next run only moves when the schedule itself moved. Changing a
        // recipient list must not push the run time forward.
        if ($schedule->cron_expression !== $previousCron || $schedule->timezone !== $previousTimezone) {
            $schedule->update([
                'next_run_at' => $this->calculator->next(
                    (string) $schedule->cron_expression,
                    $schedule->timezone ?: 'Asia/Jakarta',
                ),
            ]);
        }

        return response()->json(['schedule' => $schedule->fresh()]);
    }

    public function destroy(ReportSchedule $schedule): JsonResponse
    {
        // Hard delete. generated_reports.schedule_id is nullOnDelete, so the
        // report history survives with its schedule link cleared.
        $schedule->delete();

        return response()->json(['ok' => true]);
    }

    /**
     * @return array<int, mixed>
     */
    private function cronRules(): array
    {
        return [
            'required',
            'string',
            'max:60',
            function ($attribute, $value, $fail) {
                if (! CronExpression::isValidExpression((string) $value)) {
                    $fail('The cron expression is not valid. Use five fields, for example 0 8 * * *.');
                }
            },
        ];
    }

    /**
     * Manual "Run and send now" trigger.
     *
     * With the sync queue the report is built inside this request, so the job's
     * outcome (created, skipped, or failed) is returned directly. The UI reports what
     * actually happened instead of always claiming success (R-26, R-35).
     */
    public function runNow(ReportSchedule $schedule): JsonResponse
    {
        try {
            $result = app()->call([new GenerateAndSendReportJob($schedule->id), 'handle']);
        } catch (\Throwable $e) {
            \Illuminate\Support\Facades\Log::error('Report run-now failed', [
                'schedule_id' => $schedule->id,
                'error' => $e->getMessage(),
            ]);
            return response()->json([
                'ok' => false,
                'message' => 'Report generation failed: '.$e->getMessage(),
            ], 502);
        }
        return response()->json([
            'ok' => $result['outcome'] !== 'failed',
            'outcome' => $result['outcome'],
            'message' => $result['message'],
            'sent' => $result['sent'] ?? null,
            'failed' => $result['failed'] ?? null,
        ]);
    }

    public function download(GeneratedReport $report, string $format)
    {
        $path = $format === 'xlsx' ? $report->excel_file_path : $report->pdf_file_path;

        abort_unless($path && \Storage::disk('reports')->exists($path), 404);

        return \Storage::disk('reports')->download($path);
    }
}
