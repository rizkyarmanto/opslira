<?php

namespace Database\Seeders;

use App\Modules\Reporting\Models\ReportSchedule;
use Illuminate\Database\Seeder;

class ReportScheduleSeeder extends Seeder
{
    public function run(): void
    {
        $schedules = [
            [
                'name' => 'Daily Operations Report',
                'period_type' => 'DAILY',
                'cron_expression' => '0 8 * * *',
                'report_formats' => ['HTML', 'PDF', 'XLSX'],
            ],
            [
                'name' => 'Weekly Performance Report',
                'period_type' => 'WEEKLY',
                'cron_expression' => '0 8 * * 1',
                'report_formats' => ['PDF', 'XLSX'],
            ],
            [
                'name' => 'Monthly Executive SLA Report',
                'period_type' => 'MONTHLY',
                'cron_expression' => '0 8 1 * *',
                'report_formats' => ['PDF', 'XLSX'],
            ],
        ];

        foreach ($schedules as $schedule) {
            ReportSchedule::updateOrCreate(
                ['name' => $schedule['name']],
                [
                    ...$schedule,
                    'timezone' => 'Asia/Jakarta',
                    'scope_filter' => ['dashboard_groups' => ['ALL'], 'environments' => ['ALL']],
                    'recipients_to' => [],
                    'recipients_cc' => [],
                    'next_run_at' => now()->addDay(),
                    'is_active' => true,
                ],
            );
        }
    }
}
