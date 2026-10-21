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
    private const TOKEN_CACHE_KEY = 'beszel:token';

    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'BESZEL_API';
    }

    public function execute(): ProbeResult
    {
        $hubUrl = rtrim(trim((string) ($this->config('hub_url') ?: $this->config('host'))), '/');
        $systemId = $this->config('beszel_system_id');

        if (! $hubUrl || ! $systemId) {
            return ProbeResult::down('Missing hub_url or beszel_system_id');
        }

        $apiToken = (string) $this->credential('api_token', '');
        $email = (string) $this->credential('email', '');
        $password = (string) $this->credential('password', '');
        if ($apiToken === '' && ($email === '' || $password === '')) {
            return ProbeResult::down('Missing Beszel credentials');
        }

        try {
            $auth = $this->authToken($hubUrl);
            $token = $auth['token'];
            if (! $token) {
                return ProbeResult::down('Beszel authentication failed: '.($auth['error'] ?? 'no token returned'));
            }

            // System status
            $system = $this->authedGet($token)
                ->get("{$hubUrl}/api/collections/systems/records/{$systemId}");

            if ($system->failed()) {
                return ProbeResult::down("Beszel systems query failed: HTTP {$system->status()}");
            }

            // Latest stats
            $stats = $this->authedGet($token)
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
        } catch (ConnectionException $e) {
            return ProbeResult::down("Beszel hub unreachable at {$hubUrl}: ".$e->getMessage());
        } catch (\Throwable $e) {
            return ProbeResult::down("Beszel probe error at {$hubUrl}: ".$e->getMessage());
        }
    }

    /**
     * Build an authorized GET prelude: PocketBase takes the raw token in the
     * Authorization header (no Bearer prefix), and every call carries the probe
     * timeout. docs: Authorization:YOUR_AUTH_TOKEN
     */
    private function authedGet(string $token): \Illuminate\Http\Client\PendingRequest
    {
        return Http::withHeaders(['Authorization' => $token])
            ->timeout($this->timeoutSeconds);
    }

    /**
     * Returns token + error so the caller can report the HTTP status and body
     * instead of a bare "authentication failed". Failures are cached for at most
     * a minute: one wrong password must not lock the target out for hours.
     *
     * @return array{token: ?string, error: ?string}
     */
    private function authToken(string $hubUrl): array
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
        $cacheKey = self::TOKEN_CACHE_KEY.':'.md5($hubUrl).':'.md5($email);
        $cached = Cache::get($cacheKey);
        if (is_array($cached)) {
            return [
                'token' => $cached['token'] ?? null,
                'error' => $cached['error'] ?? null,
            ];
        }

        $response = Http::timeout($this->timeoutSeconds)
            ->post("{$hubUrl}/api/collections/users/auth-with-password", [
                'identity' => $email,
                'password' => $password,
            ]);

        if ($response->successful() && $response->json('token')) {
            $auth = ['token' => (string) $response->json('token'), 'error' => null];
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
