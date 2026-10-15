<?php

namespace App\Modules\Reporting\Builders;

use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\View;

/**
 * Renders a Blade HTML report template and converts it to PDF via Gotenberg.
 * Light theme, static SVG (no JS animation).
 * docs/referensi-design.md §6.1
 */
class PdfReportBuilder
{
    /**
     * @param  array<string, mixed>  $data
     * @return string  Stored PDF path
     */
    public function build(array $data, string $template = 'reports.daily'): string
    {
        $html = View::make($template, $data)->render();

        $gotenbergUrl = rtrim((string) config('services.gotenberg.url'), '/');

        $response = Http::timeout(120)
            ->attach('files', $html, 'index.html')
            ->post("{$gotenbergUrl}/forms/chromium/convert/html", [
                'marginTop' => '0.59',
                'marginBottom' => '0.59',
                'marginLeft' => '0.59',
                'marginRight' => '0.59',
                'printBackground' => 'true',
            ]);

        if ($response->failed()) {
            throw new \RuntimeException('Gotenberg PDF generation failed: HTTP '.$response->status());
        }

        $directory = 'reports/'.now()->format('Y-m');
        $filename = sprintf('%s-%s.pdf', $data['period_type'] ?? 'report', now()->format('Ymd-His'));

        \Storage::disk('reports')->put("{$directory}/{$filename}", $response->body());

        return "{$directory}/{$filename}";
    }
}
