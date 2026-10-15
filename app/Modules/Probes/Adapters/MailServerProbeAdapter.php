<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;

/**
 * MAIL_SERVER: SMTP/IMAP socket banner + DNSBL reverse-IP reputation check.
 */
class MailServerProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'SMTP_DNSBL';
    }

    public function execute(): ProbeResult
    {
        $host = $this->config('host');
        if (! $host) {
            return ProbeResult::down('Missing host in config_payload');
        }

        $ports = (array) $this->config('check_ports', ['smtp' => 587, 'imap' => 993]);
        $metrics = [];
        $errors = [];
        $start = microtime(true);

        foreach ($ports as $service => $port) {
            $banner = $this->probeBanner($host, (int) $port);
            $metrics["{$service}_{$port}_up"] = $banner['up'];
            if ($banner['banner']) {
                $metrics["{$service}_banner"] = $banner['banner'];
            }
            if (! $banner['up']) {
                $errors[] = "{$service}:{$port} unreachable";
            }
        }

        if ((bool) $this->config('check_dnsbl', false)) {
            $metrics = [...$metrics, ...$this->dnsblCheck($host)];
        }

        $latency = round((microtime(true) - $start) * 1000, 2);

        if (! empty($errors)) {
            return ProbeResult::down(implode('; ', $errors), $metrics, $latency);
        }

        if (($metrics['is_blacklisted'] ?? false) === true) {
            return ProbeResult::degraded($latency, $metrics, 'IP listed on DNSBL');
        }

        return new ProbeResult($this->classifyLatency($latency), $latency, null, $metrics);
    }

    /**
     * @return array{up: bool, banner: ?string}
     */
    private function probeBanner(string $host, int $port): array
    {
        $socket = @fsockopen($host, $port, $errno, $errstr, $this->timeoutSeconds);
        if (! $socket) {
            return ['up' => false, 'banner' => null];
        }

        stream_set_timeout($socket, $this->timeoutSeconds);
        $banner = @fgets($socket, 512);
        fclose($socket);

        return ['up' => true, 'banner' => $banner ? trim($banner) : null];
    }

    private function dnsblCheck(string $host): array
    {
        $ip = gethostbyname($host);
        if ($ip === $host) {
            return ['is_blacklisted' => false, 'listed_on' => []];
        }

        $providers = (array) $this->config('dnsbl_providers', ['zen.spamhaus.org', 'bl.spamcop.net']);
        $reversed = implode('.', array_reverse(explode('.', $ip)));
        $listedOn = [];

        foreach ($providers as $provider) {
            $lookup = "{$reversed}.{$provider}";
            if (gethostbyname($lookup) !== $lookup) {
                $listedOn[] = $provider;
            }
        }

        return ['is_blacklisted' => ! empty($listedOn), 'listed_on' => $listedOn];
    }
}
