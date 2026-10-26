<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;

/**
 * BESZEL_SERVER: PocketBase REST API integration.
 * Authenticates either with a static API token, or against the regular-user
 * /api/collections/users/auth-with-password endpoint (a Beszel regular user is
 * the read-only role), caches the JWT in Redis for 12 hours keyed per hub+email,
 * then pulls system + stats records. docs/gemini/dokumen_arsitektur_aplikasi_web.md:121-125
 */
class BeszelApiProbeAdapter extends BaseProbeAdapter
{
    private const TOKEN_CACHE_PREFIX = 'beszel:token';
    public static function forgetToken(string $hubUrl, string $email): void
    {
        Cache::forget(self::TOKEN_CACHE_PREFIX.':'.md5($hubUrl).':'.md5($email));
    }
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'BESZEL_API';
    }
    public function execute(): ProbeResult
    {
        $hubUrl = rtrim(trim((string) ($this->config('hub_url') ?: $this->config('host'))), '/');
        $systemName = (string) ($this->config('beszel_system_name') ?: '');
        if (! $hubUrl || $systemName === '') {
            return ProbeResult::down('Missing hub_url or beszel_system_name');
        }

        $apiToken = (string) $this->credential('api_token', '');
        $email = (string) $this->credential('email', '');
        $password = (string) $this->credential('password', '');
        if ($apiToken === '' && ($email === '' || $password === '')) {
            return ProbeResult::down('Missing Beszel credentials');
        }

        $calls = $apiToken !== '' ? 2 : 3;
        $perCall = $this->callBudget($calls);

        try {
            try {
                $auth = $this->authToken($hubUrl, $perCall);
            } catch (ConnectionException $e) {
                return ProbeResult::down("Beszel hub unreachable during auth at {$hubUrl} after {$perCall}s (check hub reachability or raise timeout_seconds): ".$e->getMessage());
            }
            $token = $auth['token'];
            if (! $token) {
                return ProbeResult::down('Beszel authentication failed: '.($auth['error'] ?? 'no token returned'));
            }

            // Resolve the human system name to a PocketBase record id.
            try {
                $list = $this->authedGet($token, $perCall)
                    ->get("{$hubUrl}/api/collections/systems/records", [
                        'filter' => "(name='{$systemName}')",
                        'perPage' => 5,
                    ]);
            } catch (ConnectionException $e) {
                return ProbeResult::down("Beszel hub unreachable during systems query at {$hubUrl} after {$perCall}s (check hub reachability or raise timeout_seconds): ".$e->getMessage());
            }
            if ($list->failed()) {
                return ProbeResult::down("Beszel systems query failed: HTTP {$list->status()} ".Str::limit((string) $list->body(), 300));
            }
            $systemData = $list->json('items.0', []);
            $systemId = (string) ($systemData['id'] ?? '');
            if ($systemId === '') {
                return ProbeResult::down("Beszel system '{$systemName}' not found on hub");
            }
            // Latest stats
            try {
                $stats = $this->authedGet($token, $perCall)
                    ->get("{$hubUrl}/api/collections/system_stats/records", [
                        'filter' => "(system='{$systemId}')",
                        'sort' => '-created',
                        'perPage' => 1,
                    ]);
            } catch (ConnectionException $e) {
                return ProbeResult::down("Beszel hub unreachable during stats query at {$hubUrl} after {$perCall}s (check hub reachability or raise timeout_seconds): ".$e->getMessage());
            }
            if ($stats->failed()) {
                return ProbeResult::down("Beszel stats query failed: HTTP {$stats->status()} ".Str::limit((string) $stats->body(), 300));
            }
            $record = $stats->json('items.0.stats', []);
            $info = is_array($systemData['info'] ?? null) ? $systemData['info'] : [];
            $metrics = [
                'agent_status' => $systemData['status'] ?? 'unknown',
                'cpu_pct' => (float) ($record['cpu'] ?? $info['cpu'] ?? 0),
                'mem_pct' => (float) ($record['mp'] ?? $info['mp'] ?? 0),
                'mem_used_gb' => (float) ($record['mu'] ?? 0),
                'mem_total_gb' => (float) ($record['m'] ?? 0),
                'disk_pct' => (float) ($record['dp'] ?? $info['dp'] ?? 0),
                'disk_used_gb' => (float) ($record['du'] ?? 0),
                'disk_total_gb' => (float) ($record['d'] ?? 0),
                'containers_running' => (int) ($info['ct'] ?? 0),
                'containers_unhealthy' => 0,
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

            if (! empty($breaches)) {
                return ProbeResult::degraded(null, $metrics, 'Threshold breach: '.implode(', ', $breaches));
            }

            return ProbeResult::up(null, $metrics);
        } catch (\Throwable $e) {
            return ProbeResult::down("Beszel probe error at {$hubUrl}: ".$e->getMessage());
        }
    }

    /**
     * Shared budget: timeout_seconds splits across sequential calls so the
     * total stays inside the target timeout. Minimum 2s per call.
     */
    private function callBudget(int $calls): int
    {
        return max(2, (int) ceil($this->timeoutSeconds / max(1, $calls)));
    }

    private function connectBudget(int $perCall): int
    {
        return min(3, $perCall);
    }

    /**
     * Build an authorized GET prelude: PocketBase takes the raw token in the
     * Authorization header (no Bearer prefix), and every call carries its share
     * of the probe timeout. docs: Authorization:YOUR_AUTH_TOKEN
     */
    private function authedGet(string $token, int $perCall): \Illuminate\Http\Client\PendingRequest
    {
        return Http::withHeaders(['Authorization' => $token])
            ->connectTimeout($this->connectBudget($perCall))
            ->timeout($perCall);
    }

    /**
     * Returns token + error so the caller can report the HTTP status and body
     * instead of a bare "authentication failed". Failures are cached for at most
     * a minute: one wrong password must not lock the target out for hours.
     *
     * @return array{token: ?string, error: ?string}
     */
    private function authToken(string $hubUrl, int $perCall): array
    {
        // Static API token: no login call, no cache.
        $apiToken = (string) $this->credential('api_token', '');
        if ($apiToken !== '') {
            return ['token' => $apiToken, 'error' => null];
        }

        $email = (string) $this->credential('email', '');
        $password = (string) $this->credential('password', '');
        if ($email === '' || $password === '') {
            return ['token' => null, 'error' => 'missing email or password'];
        }

        // Keyed per hub + email so two Beszel targets with different logins never
        // share a JWT.
        $cacheKey = self::TOKEN_CACHE_PREFIX.':'.md5($hubUrl).':'.md5($email);
        $cached = Cache::get($cacheKey);
        if (is_array($cached)) {
            return [
                'token' => $cached['token'] ?? null,
                'error' => $cached['error'] ?? null,
            ];
        }

        $response = Http::connectTimeout($this->connectBudget($perCall))->timeout($perCall)
            ->post("{$hubUrl}/api/collections/users/auth-with-password", [
                'identity' => $email,
                'password' => $password,
            ]);
        $authToken = $response->json('token') ?? $response->json('authToken');
        if ($response->successful() && is_string($authToken) && $authToken !== '') {
            $auth = ['token' => $authToken, 'error' => null];
            Cache::put($cacheKey, $auth, (int) config('services.beszel.token_cache_ttl', 43200));
            return $auth;
        }
        $auth = [
            'token' => null,
            'error' => "HTTP {$response->status()}: ".Str::limit((string) $response->body(), 300),
        ];
        Cache::put($cacheKey, $auth, 60);
        return $auth;
    }
}
