<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;

/**
 * BESZEL_SERVER: PocketBase REST API integration.
 * Authenticates against /api/collections/_superusers/auth-with-password,
 * caches JWT in Redis for 12 hours, then pulls system + stats records.
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:121-125
 */
class BeszelApiProbeAdapter extends BaseProbeAdapter
{
    private const TOKEN_CACHE_KEY = 'beszel:token';

    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'BESZEL_API';
    }

    public function execute(): ProbeResult
    {
        $hubUrl = rtrim((string) $this->config('hub_url', config('services.beszel.hub_url')), '/');
        $systemId = $this->config('beszel_system_id');

        if (! $hubUrl || ! $systemId) {
            return ProbeResult::down('Missing hub_url or beszel_system_id');
        }

        try {
            $token = $this->authToken($hubUrl);
            if (! $token) {
                return ProbeResult::down('Beszel authentication failed');
            }

            // System status
            $system = Http::withToken($token)
                ->timeout($this->timeoutSeconds)
                ->get("{$hubUrl}/api/collections/systems/records/{$systemId}");

            if ($system->failed()) {
                return ProbeResult::down("Beszel systems query failed: HTTP {$system->status()}");
            }

            // Latest stats
            $stats = Http::withToken($token)
                ->timeout($this->timeoutSeconds)
                ->get("{$hubUrl}/api/collections/system_stats/records", [
                    'filter' => "(system='{$systemId}')",
                    'sort' => '-created',
                    'perPage' => 1,
                ]);

            $record = $stats->json('items.0', []);
            $systemData = $system->json();

            $metrics = [
                'agent_status' => $systemData['status'] ?? 'unknown',
                'cpu_pct' => (float) ($record['cpu'] ?? 0),
                'mem_pct' => (float) ($record['memory'] ?? 0),
                'mem_used_gb' => (float) ($record['memory_used'] ?? 0),
                'mem_total_gb' => (float) ($record['memory_total'] ?? 0),
                'disk_pct' => (float) ($record['disk'] ?? 0),
                'disk_used_gb' => (float) ($record['disk_used'] ?? 0),
                'disk_total_gb' => (float) ($record['disk_total'] ?? 0),
                'containers_running' => (int) ($record['containers'] ?? 0),
                'containers_unhealthy' => (int) ($record['containers_unhealthy'] ?? 0),
            ];

            // Threshold-based DEGRADED classification
            $warnCpu = (float) $this->config('warn_cpu_pct', 85);
            $warnMem = (float) $this->config('warn_mem_pct', 90);
            $warnDisk = (float) $this->config('warn_disk_pct', 85);

            $breaches = [];
            if ($metrics['cpu_pct'] >= $warnCpu) {
                $breaches[] = "CPU {$metrics['cpu_pct']}%";
            }
            if ($metrics['mem_pct'] >= $warnMem) {
                $breaches[] = "MEM {$metrics['mem_pct']}%";
            }
            if ($metrics['disk_pct'] >= $warnDisk) {
                $breaches[] = "DISK {$metrics['disk_pct']}%";
            }
            if ($metrics['containers_unhealthy'] > 0) {
                $breaches[] = "{$metrics['containers_unhealthy']} unhealthy container(s)";
            }

            if (! empty($breaches)) {
                return ProbeResult::degraded(null, $metrics, 'Threshold breach: '.implode(', ', $breaches));
            }

            return ProbeResult::up(null, $metrics);
        } catch (\Throwable $e) {
            return ProbeResult::down('Beszel probe error: '.$e->getMessage());
        }
    }

    private function authToken(string $hubUrl): ?string
    {
        return Cache::remember(
            self::TOKEN_CACHE_KEY.':'.md5($hubUrl),
            (int) config('services.beszel.token_cache_ttl', 43200),
            function () use ($hubUrl) {
                $identity = (string) $this->credential('identity', '');
                $password = (string) $this->credential('password', '');

                if ($identity === '' || $password === '') {
                    return null;
                }

                $response = Http::timeout($this->timeoutSeconds)
                    ->post("{$hubUrl}/api/collections/_superusers/auth-with-password", [
                        'identity' => $identity,
                        'password' => $password,
                    ]);

                return $response->successful() ? $response->json('token') : null;
            },
        );
    }
}
