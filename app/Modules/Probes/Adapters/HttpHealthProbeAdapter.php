<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Support\Facades\Http;

/**
 * WEB_APP: HTTP/HTTPS health check (cURL via Guzzle) + JSON key validation.
 */
class HttpHealthProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'HTTP_HEALTH';
    }

    public function execute(): ProbeResult
    {
        // WEB_APP targets carry the full URL in host_or_endpoint (injected as
        // config 'host'); config_payload 'url' stays for hand-built payloads.
        $url = $this->config('url') ?? $this->config('host');
        if (! $url) {
            return ProbeResult::down('Missing url in config_payload');
        }

        $method = strtoupper((string) $this->config('method', 'GET'));
        $expectedStatus = (int) $this->config('expected_status', 200);
        $verifySsl = (bool) $this->config('verify_ssl', true);
        $bearer = $this->credential('bearer_token');

        try {
            $start = microtime(true);

            $request = Http::timeout($this->timeoutSeconds)
                ->withOptions(['verify' => $verifySsl])
                ->withHeaders(['User-Agent' => 'Opslira-Monitor/1.0']);

            if ($bearer) {
                $request = $request->withToken($bearer);
            } elseif ($this->credential('username')) {
                $request = $request->withBasicAuth(
                    (string) $this->credential('username'),
                    (string) $this->credential('password', ''),
                );
            }

            $response = $request->send($method, $url);
            $latency = round((microtime(true) - $start) * 1000, 2);

            $metrics = [
                'http_status' => $response->status(),
                'content_type' => $response->header('Content-Type'),
            ];

            if ($response->status() !== $expectedStatus) {
                return new ProbeResult(
                    'DOWN',
                    $latency,
                    "Expected HTTP {$expectedStatus}, got {$response->status()}",
                    $metrics,
                    $response->status(),
                );
            }

            // Optional JSON key/value assertion
            $jsonKey = $this->config('expected_json_key');
            if ($jsonKey) {
                $body = $response->json();
                $actual = is_array($body) ? data_get($body, $jsonKey) : null;
                $expectedValue = $this->config('expected_json_value');

                $metrics['json_key'] = $jsonKey;
                $metrics['json_value'] = $actual;

                if ($expectedValue !== null && (string) $actual !== (string) $expectedValue) {
                    return ProbeResult::down(
                        "JSON key '{$jsonKey}' = '{$actual}', expected '{$expectedValue}'",
                        $metrics,
                        $latency,
                    );
                }
            }

            return new ProbeResult($this->classifyLatency($latency), $latency, null, $metrics, $response->status());
        } catch (\Throwable $e) {
            return ProbeResult::down('HTTP probe error: '.$e->getMessage());
        }
    }
}
