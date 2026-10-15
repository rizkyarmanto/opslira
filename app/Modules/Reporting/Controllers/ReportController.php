<?php

namespace App\Modules\Reporting\Controllers;

use App\Modules\Reporting\Jobs\GenerateAndSendReportJob;
use App\Modules\Reporting\Models\EmailDeliveryLog;
use App\Modules\Reporting\Models\GeneratedReport;
use App\Modules\Reporting\Models\ReportSchedule;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class ReportController
{
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
            'cron_expression' => ['required', 'string', 'max:60'],
            'timezone' => ['string', 'max:50'],
            'recipients_to' => ['array'],
            'recipients_cc' => ['array'],
            'send_only_on_issues' => ['boolean'],
            'report_formats' => ['array'],
        ]);

        $schedule = ReportSchedule::create([
            ...$data,
            'next_run_at' => now()->addDay(),
            'is_active' => true,
        ]);

        return response()->json(['schedule' => $schedule], 201);
    }

    public function update(Request $request, ReportSchedule $schedule): JsonResponse
    {
        $schedule->update($request->only([
            'name', 'cron_expression', 'timezone', 'recipients_to',
            'recipients_cc', 'send_only_on_issues', 'report_formats', 'is_active',
        ]));

        return response()->json(['schedule' => $schedule->fresh()]);
    }

    public function destroy(ReportSchedule $schedule): JsonResponse
    {
        $schedule->update(['is_active' => false]);

        return response()->json(['ok' => true]);
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
            $result = GenerateAndSendReportJob::dispatchSync($schedule->id);
        } catch (\Throwable $e) {
            report($e);

            return response()->json([
                'ok' => false,
                'message' => 'Report generation failed: '.$e->getMessage(),
            ], 502);
        }

        return response()->json([
            'ok' => $result['outcome'] !== 'failed',
            'outcome' => $result['outcome'],
            'message' => $result['message'],
        ]);
    }

    public function download(GeneratedReport $report, string $format)
    {
        $path = $format === 'xlsx' ? $report->excel_file_path : $report->pdf_file_path;

        abort_unless($path && \Storage::disk('reports')->exists($path), 404);

        return \Storage::disk('reports')->download($path);
    }
}
