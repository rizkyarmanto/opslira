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
        try {
            $html = View::make($template, $data)->render();
        } catch (\Throwable $e) {
            throw new \RuntimeException("PDF template render failed for template {$template}: ".$e->getMessage(), 0, $e);
        }
        $gotenbergUrl = rtrim((string) config('services.gotenberg.url'), '/');
        try {
            $response = Http::timeout(120)
                ->attach('files', $html, 'index.html')
                ->post("{$gotenbergUrl}/forms/chromium/convert/html", [
                    'marginTop' => '0.59',
                    'marginBottom' => '0.59',
                    'marginLeft' => '0.59',
                    'marginRight' => '0.59',
                    'printBackground' => 'true',
                ]);
        } catch (\Throwable $e) {
            throw new \RuntimeException("Gotenberg at {$gotenbergUrl} unreachable: ".$e->getMessage(), 0, $e);
        }
        if ($response->failed()) {
            throw new \RuntimeException("PDF build failed (Gotenberg at {$gotenbergUrl}): HTTP ".$response->status());
        }
        $directory = 'reports/'.now()->format('Y-m');
        $filename = sprintf('%s-%s.pdf', $data['period_type'] ?? 'report', now()->format('Ymd-His'));
        $stored = \Storage::disk('reports')->put("{$directory}/{$filename}", $response->body());
        if (! $stored) {
            throw new \RuntimeException('PDF storage failed on disk reports.');
        }
        return "{$directory}/{$filename}";
    }
}
