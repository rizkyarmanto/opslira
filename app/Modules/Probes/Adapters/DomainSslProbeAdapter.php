<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Support\Facades\Cache;

/**
 * DOMAIN: WHOIS expiry + SSL certificate expiry via OpenSSL stream + DNS lookup.
 */
class DomainSslProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'WHOIS_SSL';
    }

    public function execute(): ProbeResult
    {
        $domain = $this->config('domain') ?: $this->config('host');
        if (! $domain) {
            return ProbeResult::down('Missing domain in config_payload');
        }

        $sslPort = (int) $this->config('check_ssl_port', 443);
        $warnDays = (int) $this->config('warn_days_before', 14);

        $metrics = [];
        $errors = [];

        // DNS resolution
        $ips = @dns_get_record($domain, DNS_A) ?: [];
        $resolvedIp = $ips[0]['ip'] ?? null;
        $metrics['dns_resolved_ip'] = $resolvedIp;
        if (! $resolvedIp) {
            $errors[] = 'DNS A record not found';
        }

        // SSL certificate expiry
        $sslCheck = $this->sslDaysRemaining($domain, $sslPort);
        $sslDaysLeft = $sslCheck['days'];
        if ($sslDaysLeft !== null) {
            $metrics['ssl_days_left'] = $sslDaysLeft;
            $metrics['ssl_expiry_date'] = now()->addDays($sslDaysLeft)->toDateString();
        } else {
            $errors[] = $sslCheck['error'] ?? 'SSL handshake failed';
        }

        // WHOIS expiry (best-effort)
        $whois = $this->whoisExpiry($domain);
        if ($whois['days_left'] !== null) {
            $metrics['domain_days_left'] = $whois['days_left'];
            $metrics['domain_expiry_date'] = $whois['expiry_date'];
            $metrics['domain_status'] = 'active';
        }
        if ($whois['error'] !== null) {
            // Best-effort check: the reason goes into the metrics, not into the
            // DOWN message, so a silent whois server never fails the target.
            $metrics['whois_error'] = $whois['error'];
        }

        if (! empty($errors)) {
            return ProbeResult::down(implode('; ', $errors), $metrics);
        }

        // DEGRADED when SSL or domain is close to expiry
        $minDays = min(
            $sslDaysLeft ?? PHP_INT_MAX,
            $whois['days_left'] ?? PHP_INT_MAX,
        );

        if ($minDays <= $warnDays) {
            return ProbeResult::degraded(null, $metrics, "Expiry within {$warnDays} days (min: {$minDays}d)");
        }

        return ProbeResult::up(null, $metrics);
    }

    /**
     * @return array{days: ?int, error: ?string}
     */
    private function sslDaysRemaining(string $domain, int $port): array
    {
        try {
            $context = stream_context_create([
                'ssl' => ['capture_peer_cert' => true, 'verify_peer' => false, 'verify_peer_name' => false],
            ]);

            $client = @stream_socket_client(
                "ssl://{$domain}:{$port}",
                $errno,
                $errstr,
                $this->timeoutSeconds,
                STREAM_CLIENT_CONNECT,
                $context,
            );

            if (! $client) {
                return ['days' => null, 'error' => 'SSL handshake failed: '.($errstr !== '' ? $errstr : "errno {$errno}")];
            }

            $params = stream_context_get_params($client);
            fclose($client);

            $cert = $params['options']['ssl']['peer_certificate'] ?? null;
            if (! $cert) {
                return ['days' => null, 'error' => 'SSL handshake failed: no peer certificate offered'];
            }

            $info = openssl_x509_parse($cert);
            $validTo = $info['validTo_time_t'] ?? null;

            if (! $validTo) {
                return ['days' => null, 'error' => 'SSL handshake failed: certificate has no expiry date'];
            }

            return ['days' => (int) floor(($validTo - time()) / 86400), 'error' => null];
        } catch (\Throwable $e) {
            return ['days' => null, 'error' => 'SSL handshake failed: '.$e->getMessage()];
        }
    }

    /**
     * @return array{days_left: ?int, expiry_date: ?string, error: ?string}
     */
    private function whoisExpiry(string $domain): array
    {
        // Rate-limit WHOIS to once per 12h per domain; the probe interval floor
        // is far lower and a whois server should not be hammered every check.
        return Cache::remember('probe:whois:'.sha1($domain), 43200, function () use ($domain): array {
            try {
                $tld = substr(strrchr($domain, '.'), 1);
                $server = $this->config('whois_server') ?: "whois.{$tld}";

                $socket = @fsockopen($server, 43, $errno, $errstr, $this->timeoutSeconds);
                if (! $socket) {
                    return [
                        'days_left' => null,
                        'expiry_date' => null,
                        'error' => 'WHOIS lookup failed: '.($errstr !== '' ? $errstr : "errno {$errno}"),
                    ];
                }

                fwrite($socket, $domain."\r\n");
                $response = '';
                while (! feof($socket)) {
                    $response .= fgets($socket, 128);
                }
                fclose($socket);

                if (preg_match('/Expiry Date:\s*(.+)/i', $response, $m)
                    || preg_match('/Registry Expiry Date:\s*(.+)/i', $response, $m)
                    || preg_match('/expires:\s*(.+)/i', $response, $m)) {
                    $expiry = strtotime(trim($m[1]));
                    if ($expiry) {
                        return [
                            'days_left' => (int) floor(($expiry - time()) / 86400),
                            'expiry_date' => date('Y-m-d', $expiry),
                            'error' => null,
                        ];
                    }
                }

                return [
                    'days_left' => null,
                    'expiry_date' => null,
                    'error' => 'WHOIS lookup failed: no expiry date in the reply from '.$server,
                ];
            } catch (\Throwable $e) {
                return ['days_left' => null, 'expiry_date' => null, 'error' => 'WHOIS lookup failed: '.$e->getMessage()];
            }
        });
    }
}
