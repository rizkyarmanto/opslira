<?php

namespace App\Modules\Reporting\Builders;

use Illuminate\Support\Facades\Storage;

/**
 * Builds an Excel/CSV report of aggregated monitoring data.
 * Uses a dependency-free CSV writer (openable in Excel / LibreOffice).
 * (maatwebsite/excel or phpspreadsheet can be swapped in later.)
 */
class ExcelReportBuilder
{
    /**
     * @param  array<string, mixed>  $data
     * @return string  Stored file path
     */
    public function build(array $data): string
    {
        $directory = 'reports/'.now()->format('Y-m');
        $filename = sprintf('%s-%s.csv', $data['period_type'] ?? 'report', now()->format('Ymd-His'));

        $handle = fopen('php://temp', 'r+');

        // Sheet 1: Summary
        fputcsv($handle, ['Summary']);
        fputcsv($handle, ['Period', $data['period_label'] ?? '-']);
        fputcsv($handle, ['Global SLA (%)', $data['summary']['global_sla'] ?? '-']);
        fputcsv($handle, ['Total Incidents', $data['summary']['total_incidents'] ?? 0]);
        fputcsv($handle, ['Total Downtime (minutes)', round(($data['summary']['total_downtime_seconds'] ?? 0) / 60, 1)]);
        fputcsv($handle, ['MTTR (minutes)', round(($data['summary']['mttr_seconds'] ?? 0) / 60, 1)]);
        fputcsv($handle, []);

        // Sheet 2: SLA per target
        fputcsv($handle, ['Target', 'Group', 'Environment', 'SLA (%)', 'Incidents', 'Downtime (s)']);
        foreach ($data['rows'] ?? [] as $row) {
            fputcsv($handle, [
                $row['name'],
                $row['dashboard_group'],
                $row['environment'],
                $row['uptime_percentage'],
                $row['incident_count'],
                $row['total_downtime_seconds'],
            ]);
        }

        rewind($handle);
        $csv = stream_get_contents($handle);
        fclose($handle);

        Storage::disk('reports')->put("{$directory}/{$filename}", $csv);

        return "{$directory}/{$filename}";
    }
}
