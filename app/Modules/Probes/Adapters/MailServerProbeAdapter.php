<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;

/**
 * MAIL_SERVER: SMTP/IMAP socket banner + DNSBL reverse-IP reputation check.
 *
 * Encryption is negotiated explicitly: implicit TLS on 465/993, STARTTLS on the
 * other ports. AUTH runs only when the server advertises it, and only over an
 * upgraded connection unless the operator chose "none", so a refused upgrade or
 * a missing extension is reported as itself instead of looking like a bad
 * password.
 */
class MailServerProbeAdapter extends BaseProbeAdapter
{
    /** Explicit choices from the form; anything else is inferred from the port. */
    private const ENCRYPTION_CHOICES = ['auto', 'starttls', 'ssl', 'none'];

    /**
     * Crypto methods the upgrade accepts. TLS 1.3 negotiates through its own
     * method in PHP, so it is listed explicitly instead of relying on the shape
     * of STREAM_CRYPTO_METHOD_TLS_CLIENT (on PHP 8.4 that mask is 121 and already
     * contains the 1.3 bit, 65).
     */
    private const TLS_METHODS = STREAM_CRYPTO_METHOD_TLS_CLIENT | STREAM_CRYPTO_METHOD_TLSv1_3_CLIENT;

    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'SMTP_DNSBL';
    }

    public function execute(): ProbeResult
    {
        $host = (string) $this->config('host');
        if ($host === '') {
            return ProbeResult::down('Missing host in config_payload');
        }

        ['mode' => $service, 'port' => $port, 'encryption' => $encryption] = $this->mailServicePort();
        $metrics = ['encryption_mode' => $encryption];
        $errors = [];
        $start = microtime(true);

        $probe = $service === 'smtp'
            ? $this->probeSmtp($host, $port, $encryption)
            : $this->probeImap($host, $port, $encryption);

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
     * Single service, port and encryption to probe. Port precedence: shared Port
     * column, legacy check_ports, legacy smtp_port/imap_port keys (rows saved by
     * the previous form keep their port), then the 587/993 defaults.
     *
     * @return array{mode: string, port: int, encryption: string}
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

        $encryption = (string) ($this->config('mail_encryption') ?: 'auto');
        if (! in_array($encryption, self::ENCRYPTION_CHOICES, true)) {
            $encryption = 'auto';
        }
        if ($encryption === 'auto') {
            // 465 and 993 speak TLS from the first byte; every other port starts
            // in clear text and upgrades with STARTTLS when the server offers it.
            $implicitPort = $mode === 'imap' ? 993 : 465;
            $encryption = $port === $implicitPort ? 'ssl' : 'starttls';
        }

        return ['mode' => $mode, 'port' => $port, 'encryption' => $encryption];
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
     * The probe itself negotiates EHLO directly, because only a real EHLO reply
     * carries the STARTTLS and AUTH extensions.
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
     * SMTP probe: 220 greeting, EHLO, STARTTLS upgrade, then AUTH when the server
     * advertises it and credentials are configured.
     *
     * @return array{up: bool, banner: ?string, error: ?string, metrics: array<string, mixed>}
     */
    private function probeSmtp(string $host, int $port, string $encryption): array
    {
        $implicitTls = $encryption === 'ssl';
        $connection = $this->openSocket($host, $port, $encryption);
        $socket = $connection['socket'];
        if (! $socket) {
            return ['up' => false, 'banner' => null, 'error' => 'connect failed: '.$connection['error'], 'metrics' => []];
        }

        $metrics = [];

        // A greeting may be multiline ("220-mail ... 220 ..."), and reading one
        // line of it leaves the rest queued to be misread as the EHLO reply.
        $greeting = $this->smtpReadReply($socket);
        $banner = $greeting !== null ? trim($greeting) : null;
        if ($banner === null || $this->smtpReplyCode($banner) !== 220) {
            fclose($socket);

            return ['up' => false, 'banner' => $banner, 'error' => null, 'metrics' => []];
        }

        $ehlo = $this->smtpDialogue($socket, 'EHLO '.$host);
        if ($ehlo === null || $this->smtpReplyCode($ehlo) !== 250) {
            $helo = $this->smtpDialogue($socket, 'HELO '.$host);
            if ($helo === null || $this->smtpReplyCode($helo) !== 250) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'EHLO failed'.($ehlo !== null ? ': '.trim($ehlo) : ''), 'metrics' => $metrics];
            }

            // HELO carries no extensions, so STARTTLS and AUTH cannot be
            // negotiated on this session.
            $metrics['starttls_available'] = false;
            $metrics['auth_advertised'] = false;
            @fwrite($socket, "QUIT\r\n");
            fclose($socket);

            return [
                'up' => true,
                'banner' => $banner,
                'error' => 'EHLO failed, HELO fallback: extensions unavailable',
                'metrics' => $metrics,
            ];
        }

        $starttlsAdvertised = stripos($ehlo, 'STARTTLS') !== false;
        $metrics['starttls_available'] = $starttlsAdvertised;

        if (! $implicitTls && $starttlsAdvertised) {
            $reply = $this->smtpDialogue($socket, 'STARTTLS');
            if ($reply === null || $this->smtpReplyCode($reply) !== 220) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS rejected', 'metrics' => $metrics];
            }
            if (! $this->enableTls($socket, $host)) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS upgrade failed', 'metrics' => $metrics];
            }
            $metrics['starttls_used'] = true;

            // The extension list changes after the upgrade: relays commonly
            // advertise AUTH only on the encrypted session.
            $ehlo = $this->smtpDialogue($socket, 'EHLO '.$host);
            if ($ehlo === null || $this->smtpReplyCode($ehlo) !== 250) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'EHLO after STARTTLS failed'.($ehlo !== null ? ': '.trim($ehlo) : ''), 'metrics' => $metrics];
            }
        } elseif (! $implicitTls && $encryption === 'starttls' && $this->encryptionRequired()) {
            fclose($socket);

            return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS not advertised by server', 'metrics' => $metrics];
        }

        $authMechanisms = $this->smtpAuthMechanisms((string) $ehlo);
        $authAdvertised = $authMechanisms !== [];
        $metrics['auth_advertised'] = $authAdvertised;

        $username = $this->credential('smtp_username');
        $password = $this->credential('smtp_password');
        $hasCredentials = is_string($username) && $username !== '' && is_string($password) && $password !== '';

        if ($hasCredentials && ! $authAdvertised) {
            fclose($socket);

            return ['up' => true, 'banner' => $banner, 'error' => 'SMTP AUTH not advertised by server', 'metrics' => $metrics];
        }

        if ($hasCredentials) {
            $reply = $this->smtpAuthLogin($socket, $username, $password);
            $code = $reply !== null ? $this->smtpReplyCode($reply) : null;

            // 504/535: the server may refuse LOGIN and only accept PLAIN.
            if (in_array($code, [504, 535], true) && in_array('PLAIN', $authMechanisms, true)) {
                $reply = $this->smtpAuthPlain($socket, $username, $password);
                $code = $reply !== null ? $this->smtpReplyCode($reply) : null;
            }

            // 503 "bad sequence of commands" is what a server answers when the
            // session is already authenticated.
            $ok = $code === 235 || $code === 503;
            $metrics['smtp_auth_ok'] = $ok;

            if (! $ok) {
                fclose($socket);

                return [
                    'up' => true,
                    'banner' => $banner,
                    'error' => 'SMTP AUTH failed: '.($reply ?? 'no reply from server'),
                    'metrics' => $metrics,
                ];
            }
        }

        @fwrite($socket, "QUIT\r\n");
        fclose($socket);

        return ['up' => true, 'banner' => $banner, 'error' => null, 'metrics' => $metrics];
    }

    /**
     * IMAP probe: untagged "* OK" greeting, STARTTLS on a clear-text port when
     * the greeting advertises it, then LOGIN when credentials are configured.
     * IMAP has no pre-login capability list beyond the greeting, so the port
     * decides implicit TLS (993) and everything else upgrades.
     *
     * @return array{up: bool, banner: ?string, error: ?string, metrics: array<string, mixed>}
     */
    private function probeImap(string $host, int $port, string $encryption): array
    {
        $implicitTls = $encryption === 'ssl';
        $connection = $this->openSocket($host, $port, $encryption);
        $socket = $connection['socket'];
        if (! $socket) {
            return ['up' => false, 'banner' => null, 'error' => 'connect failed: '.$connection['error'], 'metrics' => []];
        }

        $metrics = [];

        $tag = 'a001';
        $greeting = $this->imapReadLine($socket);
        $banner = $greeting !== null ? trim($greeting) : null;
        if ($banner === null || ! str_starts_with($banner, '* OK')) {
            fclose($socket);

            return ['up' => false, 'banner' => $banner, 'error' => null, 'metrics' => []];
        }

        if (! $implicitTls && $encryption !== 'none') {
            $starttlsAdvertised = stripos($banner, 'STARTTLS') !== false;
            $metrics['starttls_available'] = $starttlsAdvertised;

            if ($starttlsAdvertised) {
                if (@fwrite($socket, $tag." STARTTLS\r\n") === false) {
                    fclose($socket);

                    return ['up' => true, 'banner' => $banner, 'error' => 'STARTTLS request could not be sent', 'metrics' => $metrics];
                }
                $line = $this->imapReadTaggedLine($socket, $tag);
                if ($line === null || ! $this->imapReplyIsOk($line, $tag)) {
                    fclose($socket);

                    return ['up' => true, 'banner' => $banner, 'error' => 'IMAP STARTTLS rejected'.($line !== null ? ': '.trim($line) : ''), 'metrics' => $metrics];
                }
                if (! $this->enableTls($socket, $host)) {
                    fclose($socket);

                    return ['up' => true, 'banner' => $banner, 'error' => 'IMAP STARTTLS upgrade failed', 'metrics' => $metrics];
                }
                $metrics['starttls_used'] = true;
            } elseif ($this->encryptionRequired()) {
                fclose($socket);

                return ['up' => true, 'banner' => $banner, 'error' => 'IMAP STARTTLS not advertised by server', 'metrics' => $metrics];
            }
        }

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

    /**
     * Whether the operator asked for TLS on this target rather than letting the
     * port decide: an inferred upgrade stays optional, so a banner-only check
     * against a plain port still reports UP.
     */
    private function encryptionRequired(): bool
    {
        return in_array((string) $this->config('mail_encryption'), ['starttls', 'ssl'], true);
    }

    /**
     * Stream context with certificate verification on. Without it PHP falls back
     * to its defaults and a certificate that does not match the host passes.
     *
     * @return array<string, array<string, mixed>>
     */
    private function streamContextOptions(string $host): array
    {
        return [
            'ssl' => [
                'peer_name' => $host,
                'SNI_enabled' => true,
                'verify_peer' => true,
                'verify_peer_name' => true,
                'allow_self_signed' => false,
            ],
        ];
    }

    /**
     * Open the probe socket with the certificate verification context attached.
     * fsockopen cannot take a context (its arguments stop at the timeout), so the
     * connect goes through stream_socket_client.
     *
     * @return array{socket: ?resource, error: string}
     */
    private function openSocket(string $host, int $port, string $encryption): array
    {
        $address = ($encryption === 'ssl' ? 'ssl://' : '').$host;

        $socket = @stream_socket_client(
            "{$address}:{$port}",
            $errno,
            $errstr,
            $this->timeoutSeconds,
            STREAM_CLIENT_CONNECT,
            $this->streamContext($host),
        );

        if (! $socket) {
            return ['socket' => null, 'error' => ! empty($errstr) ? $errstr : "errno {$errno}"];
        }

        stream_set_timeout($socket, $this->timeoutSeconds);

        return ['socket' => $socket, 'error' => ''];
    }

    /**
     * @return resource
     */
    private function streamContext(string $host)
    {
        return stream_context_create($this->streamContextOptions($host));
    }

    /**
     * Upgrade a clear-text socket to TLS, re-applying the verification context
     * because the upgrade performs its own handshake.
     *
     * @param  resource  $socket
     */
    private function enableTls($socket, string $host): bool
    {
        foreach ($this->streamContextOptions($host)['ssl'] as $option => $value) {
            stream_context_set_option($socket, 'ssl', $option, $value);
        }

        return (bool) stream_socket_enable_crypto($socket, true, self::TLS_METHODS);
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
     * AUTH mechanisms from an EHLO reply, uppercased: the "250-AUTH LOGIN PLAIN"
     * and "250 AUTH=LOGIN" spellings both occur in the field.
     *
     * @return array<int, string>
     */
    private function smtpAuthMechanisms(string $ehloReply): array
    {
        if (! preg_match_all('/^250[- ]AUTH[ =]+(.+)$/mi', $ehloReply, $matches)) {
            return [];
        }

        $mechanisms = [];
        foreach ($matches[1] as $line) {
            foreach (preg_split('/\s+/', trim($line)) ?: [] as $token) {
                if ($token !== '') {
                    $mechanisms[] = strtoupper($token);
                }
            }
        }

        return array_values(array_unique($mechanisms));
    }

    /**
     * Run AUTH LOGIN and return the server's final reply, trimmed so the caller
     * can report the reason a relay gave (235 on success).
     */
    private function smtpAuthLogin($socket, string $username, string $password): ?string
    {
        $reply = $this->smtpDialogue($socket, 'AUTH LOGIN');
        if ($reply === null || $this->smtpReplyCode($reply) !== 334) {
            return $reply !== null ? trim($reply) : null;
        }

        $reply = $this->smtpDialogue($socket, base64_encode($username));
        if ($reply === null || $this->smtpReplyCode($reply) !== 334) {
            return $reply !== null ? trim($reply) : null;
        }

        $reply = $this->smtpDialogue($socket, base64_encode($password));

        return $reply !== null ? trim($reply) : null;
    }

    /**
     * AUTH PLAIN with the "authorization id, authentication id, password" payload
     * in one line, for relays that do not offer the LOGIN challenge.
     */
    private function smtpAuthPlain($socket, string $username, string $password): ?string
    {
        $reply = $this->smtpDialogue($socket, 'AUTH PLAIN '.base64_encode("\0{$username}\0{$password}"));

        return $reply !== null ? trim($reply) : null;
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
