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

        ['mode' => $service, 'port' => $port] = $this->mailServicePort();
        $metrics = [];
        $errors = [];
        $start = microtime(true);

        $probe = $service === 'smtp'
            ? $this->probeSmtp($host, $port)
            : $this->probeImap($host, $port);

        $metrics["{$service}_{$port}_up"] = $probe['up'];
        if ($probe['banner']) {
            $metrics["{$service}_banner"] = $probe['banner'];
        }
        foreach ($probe['metrics'] as $key => $value) {
            $metrics[$key] = $value;
        }
        if (! $probe['up']) {
            // Prefer the real reason (connection refused, no banner) over the
            // generic "unreachable", which says nothing to an operator.
            $errors[] = "{$service}:{$port} ".($probe['error'] ?? 'unreachable');
        } elseif ($probe['error'] !== null) {
            $errors[] = "{$service}:{$port} {$probe['error']}";
        }

        // The form sends "true"/"false" as text; a plain (bool) cast would turn
        // the string "false" on.
        if (filter_var($this->config('check_dnsbl', false), FILTER_VALIDATE_BOOLEAN)) {
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
     * Single service and port to probe. Precedence: shared Port column, legacy
     * check_ports, legacy smtp_port/imap_port keys (rows saved by the previous
     * form keep their port), then the 587/993 defaults.
     *
     * @return array{mode: string, port: int}
     */
    public function mailServicePort(): array
    {
        $mode = (string) ($this->config('mail_check_mode') ?: 'smtp');
        if (! in_array($mode, ['smtp', 'imap'], true)) {
            $mode = 'smtp';
        }

        $legacy = (array) $this->config('check_ports', []);

        $port = (int) ($this->config('port')
            ?: ($legacy[$mode] ?? null)
            ?: ($mode === 'smtp' ? $this->config('smtp_port') : $this->config('imap_port'))
            ?: ($mode === 'imap' ? 993 : 587));

        return ['mode' => $mode, 'port' => $port];
    }

    /**
     * Extract the three-digit SMTP reply code from a reply line.
     */
    public function smtpReplyCode(string $line): int
    {
        return (int) substr($line, 0, 3);
    }

    /**
     * Whether a tagged IMAP reply line is a success (e.g. "a001 OK ...").
     */
    public function imapReplyIsOk(string $line, string $tag = 'a001'): bool
    {
        return str_starts_with($line, $tag.' OK');
    }

    /**
     * Send EHLO with the target host as identity; some servers reject an
     * unresolvable .local EHLO name, so fall back to HELO on a non-250 reply.
     */
    public function smtpEhloReply($socket, string $host): ?string
    {
        $reply = $this->smtpDialogue($socket, 'EHLO '.$host);
        if ($reply !== null && $this->smtpReplyCode($reply) === 250) {
            return $reply;
        }

        return $this->smtpDialogue($socket, 'HELO '.$host);
    }

    /**
     * SMTP probe: 220 greeting, EHLO, optional STARTTLS upgrade, and optional
     * AUTH LOGIN when smtp_username/smtp_password credentials are present.
     *
     * @return array{up: bool, banner: ?string, error: ?string, metrics: array<string, mixed>}
     */
    private function probeSmtp(string $host, int $port): array
    {
        $implicitTls = $port === 465;
        $address = ($implicitTls ? 'ssl://' : '').$host;
        $socket = @fsockopen($address, $port, $errno, $errstr, $this->timeoutSeconds);
        if (! $socket) {
            return [
                'up' => false,
                'banner' => null,
                'error' => 'connect failed: '.(! empty($errstr) ? $errstr : "errno {$errno}"),
                'metrics' => [],
            ];
        }

        stream_set_timeout($socket, $this->timeoutSeconds);
        $metrics = [];

        $greeting = $this->smtpReadLine($socket);
        $banner = $greeting !== null ? trim($greeting) : null;
        if ($banner === null || $this->smtpReplyCode($banner) !== 220) {
            fclose($socket);

            return ['up' => false, 'banner' => $banner, 'error' => null, 'metrics' => []];
        }

        $ehlo = $this->smtpEhloReply($socket, $host);
        if ($ehlo === null || $this->smtpReplyCode($ehlo) !== 250) {
            fclose($socket);

            return ['up' => true, 'banner' => $banner, 'error' => 'EHLO failed'.($ehlo !== null ? ': '.trim($ehlo) : ''), 'metrics' => $metrics];
        }
        $starttlsAdvertised = stripos($ehlo, 'STARTTLS') !== false;
        $metrics['starttls_available'] = $starttlsAdvertised;

        if (! $implicitTls && $starttlsAdvertised) {
            $reply = $this->smtpDialogue($socket, 'STARTTLS');
            if ($reply === null || $this->smtpReplyCode($reply) !== 220) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS rejected', 'metrics' => $metrics];
            }
            if (! stream_socket_enable_crypto($socket, true, STREAM_CRYPTO_METHOD_TLS_CLIENT)) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS upgrade failed', 'metrics' => $metrics];
            }
            $ehlo = $this->smtpEhloReply($socket, $host);
            if ($ehlo === null || $this->smtpReplyCode($ehlo) !== 250) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'EHLO after STARTTLS failed'.($ehlo !== null ? ': '.trim($ehlo) : ''), 'metrics' => $metrics];
            }
        }

        $username = $this->credential('smtp_username');
        $password = $this->credential('smtp_password');
        if (is_string($username) && $username !== '' && is_string($password) && $password !== '') {
            $code = $this->smtpAuthLogin($socket, $username, $password);
            $ok = $code === 235;
            $metrics['smtp_auth_ok'] = $ok;
            if (! $ok) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'SMTP AUTH failed'.($code !== null ? ": {$code}" : ''), 'metrics' => $metrics];
            }
        }

        @fwrite($socket, "QUIT\r\n");
        fclose($socket);

        return ['up' => true, 'banner' => $banner, 'error' => null, 'metrics' => $metrics];
    }

    /**
     * IMAP probe: untagged "* OK" greeting, optional LOGIN when
     * imap_username/imap_password credentials are present.
     *
     * @return array{up: bool, banner: ?string, error: ?string, metrics: array<string, mixed>}
     */
    private function probeImap(string $host, int $port): array
    {
        $implicitTls = $port === 993;
        $address = ($implicitTls ? 'ssl://' : '').$host;
        $socket = @fsockopen($address, $port, $errno, $errstr, $this->timeoutSeconds);
        if (! $socket) {
            return [
                'up' => false,
                'banner' => null,
                'error' => 'connect failed: '.(! empty($errstr) ? $errstr : "errno {$errno}"),
                'metrics' => [],
            ];
        }

        stream_set_timeout($socket, $this->timeoutSeconds);
        $metrics = [];

        $greeting = $this->imapReadLine($socket);
        $banner = $greeting !== null ? trim($greeting) : null;
        if ($banner === null || ! str_starts_with($banner, '* OK')) {
            fclose($socket);

            return ['up' => false, 'banner' => $banner, 'error' => null, 'metrics' => []];
        }

        $tag = 'a001';
        $username = $this->credential('imap_username');
        $password = $this->credential('imap_password');
        if (is_string($username) && $username !== '' && is_string($password) && $password !== '') {
            @fwrite($socket, $tag.' LOGIN "'.addslashes($username).'" "'.addslashes($password).'"'."\r\n");
            $line = $this->imapReadTaggedLine($socket, $tag);
            $ok = $line !== null && $this->imapReplyIsOk($line, $tag);
            $metrics['imap_auth_ok'] = $ok;
            if (! $ok) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'IMAP LOGIN failed'.($line !== null ? ': '.trim($line) : ''), 'metrics' => $metrics];
            }
        }

        @fwrite($socket, $tag." LOGOUT\r\n");
        fclose($socket);

        return ['up' => true, 'banner' => $banner, 'error' => null, 'metrics' => $metrics];
    }

    private function smtpReadLine($socket): ?string
    {
        $line = @fgets($socket, 512);

        return $line === false ? null : $line;
    }

    /**
     * Read a full (possibly multiline) SMTP reply and return it concatenated.
     */
    private function smtpReadReply($socket): ?string
    {
        $reply = '';
        while (($line = @fgets($socket, 512)) !== false) {
            $reply .= $line;
            if (strlen($line) === 0) {
                break;
            }
            if (isset($line[3]) && $line[3] !== '-') {
                break;
            }
        }

        return $reply !== '' ? $reply : null;
    }

    private function smtpDialogue($socket, string $command): ?string
    {
        if (@fwrite($socket, $command."\r\n") === false) {
            return null;
        }

        return $this->smtpReadReply($socket);
    }

    /**
     * Run AUTH LOGIN and return the final reply code (235 on success).
     */
    private function smtpAuthLogin($socket, string $username, string $password): ?int
    {
        $reply = $this->smtpDialogue($socket, 'AUTH LOGIN');
        if ($reply === null || $this->smtpReplyCode($reply) !== 334) {
            return $reply !== null ? $this->smtpReplyCode($reply) : null;
        }

        $reply = $this->smtpDialogue($socket, base64_encode($username));
        if ($reply === null || $this->smtpReplyCode($reply) !== 334) {
            return $reply !== null ? $this->smtpReplyCode($reply) : null;
        }

        $reply = $this->smtpDialogue($socket, base64_encode($password));
        if ($reply === null) {
            return null;
        }

        return $this->smtpReplyCode($reply);
    }

    private function imapReadLine($socket): ?string
    {
        $line = @fgets($socket, 512);

        return $line === false ? null : $line;
    }

    private function imapReadTaggedLine($socket, string $tag): ?string
    {
        while (($line = @fgets($socket, 512)) !== false) {
            if (str_starts_with($line, $tag.' ')) {
                return $line;
            }
        }

        return null;
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
