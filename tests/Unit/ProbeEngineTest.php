<?php

use App\Modules\Probes\Adapters\DatabasePingProbeAdapter;
use App\Modules\Probes\Adapters\MailServerProbeAdapter;
use App\Modules\Probes\Adapters\NetworkSnmpProbeAdapter;
use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeAdapterFactory;
use App\Modules\Probes\Services\ProbeFailureLogger;
use App\Modules\Reporting\Services\SlaCalculatorService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Facades\Log;
use Spatie\Ping\PingResult;

/**
 * Call a private MailServerProbeAdapter method from a unit test. The SMTP and
 * IMAP frame readers are private, and the socket-pair tests below are the only
 * way to exercise them without a live relay.
 *
 * @param  array<int, mixed>  $arguments
 */
function mailProbeCall(object $adapter, string $method, array $arguments): mixed
{
    $reflection = new ReflectionMethod($adapter, $method);

    return $reflection->invoke($adapter, ...$arguments);
}

it('returns the correct adapter for every supported protocol', function () {
    $factory = new ProbeAdapterFactory();

    foreach ($factory->supportedProtocols() as $protocol) {
        $adapter = $factory->make($protocol);
        expect($adapter->supportsProtocol($protocol))->toBeTrue();
    }
});

it('throws for an unsupported protocol', function () {
    $factory = new ProbeAdapterFactory();
    $factory->make('UNKNOWN_PROTOCOL');
})->throws(InvalidArgumentException::class);

it('classifies probe result factories correctly', function () {
    expect(ProbeResult::up(120.0)->isUp())->toBeTrue();
    expect(ProbeResult::down('timeout')->isDown())->toBeTrue();
    expect(ProbeResult::degraded(1500.0, [], 'slow')->status)->toBe('DEGRADED');
});

it('exposes a concrete SLA calculator service', function () {
    expect(app(SlaCalculatorService::class))->toBeInstanceOf(SlaCalculatorService::class);
});

it('resolves the default port per database driver', function () {
    $configure = static fn (array $config): DatabasePingProbeAdapter => (new DatabasePingProbeAdapter())->configure($config);

    expect($configure(['host' => 'db'])->resolvePort('pgsql'))->toBe(5432)
        ->and($configure(['host' => 'db'])->resolvePort('mysql'))->toBe(3306)
        ->and($configure(['host' => 'db'])->resolvePort('mariadb'))->toBe(3306)
        ->and($configure(['host' => 'db'])->resolvePort('sqlsrv'))->toBe(1433)
        ->and($configure(['host' => 'db', 'port' => 6543])->resolvePort('mysql'))->toBe(6543);
});

it('reports an unsupported database driver instead of probing with pgsql', function () {
    $adapter = (new DatabasePingProbeAdapter())->configure([
        'host' => 'db', 'db_driver' => 'postgres', 'database_name' => 'app',
    ]);

    $result = $adapter->execute();

    expect($result->status)->toBe('DOWN')
        ->and($result->errorMessage)->toContain("Unsupported db_driver 'postgres'");
});

it('builds snmpget arguments for v2c community', function () {
    $adapter = (new NetworkSnmpProbeAdapter())
        ->configure(['enable_snmp' => true, 'host' => '10.0.0.1'], ['community' => 'xyz']);

    expect($adapter->snmpCommandArguments('10.0.0.1'))->toBe([
        'snmpget', '-v', '2c', '-c', 'xyz',
        '-t', '5', '10.0.0.1', '1.3.6.1.2.1.1.3.0',
    ]);
});

it('builds snmpget arguments for v3 authPriv', function () {
    $adapter = (new NetworkSnmpProbeAdapter())
        ->configure(['enable_snmp' => true, 'host' => '10.0.0.1'], [
            'username' => 'monuser',
            'auth_protocol' => 'SHA',
            'auth_password' => 'auth-secret',
            'priv_protocol' => 'AES',
            'priv_password' => 'priv-secret',
        ]);

    expect($adapter->snmpCommandArguments('10.0.0.1'))->toBe([
        'snmpget', '-v', '3', '-u', 'monuser', '-l', 'authPriv',
        '-a', 'SHA', '-A', 'auth-secret',
        '-x', 'AES', '-X', 'priv-secret',
        '-t', '5', '10.0.0.1', '1.3.6.1.2.1.1.3.0',
    ]);
});

it('returns no snmpget arguments when no credentials are configured', function () {
    $adapter = (new NetworkSnmpProbeAdapter())
        ->configure(['enable_snmp' => true, 'host' => '10.0.0.1'], []);

    expect($adapter->snmpCommandArguments('10.0.0.1'))->toBe([]);
});

it('builds snmpget arguments for several oids with quick numeric output', function () {
    $adapter = (new NetworkSnmpProbeAdapter())
        ->configure(['enable_snmp' => true, 'host' => '192.168.10.1'], ['community' => 'solusnmp']);

    expect($adapter->snmpGetArguments('192.168.10.1', ['1.3.6.1.2.1.1.1.0', '1.3.6.1.2.1.1.5.0']))->toBe([
        'snmpget', '-v', '2c', '-c', 'solusnmp',
        '-t', '5', '-r', '1', '-Oqn',
        '192.168.10.1', '1.3.6.1.2.1.1.1.0', '1.3.6.1.2.1.1.5.0',
    ]);
});

it('builds snmpwalk arguments for a table base oid', function () {
    $adapter = (new NetworkSnmpProbeAdapter())
        ->configure(['enable_snmp' => true, 'host' => '192.168.200.6'], ['community' => 'monitoring']);

    expect($adapter->snmpWalkArguments('192.168.200.6', '1.3.6.1.2.1.2.2.1.2'))->toBe([
        'snmpwalk', '-v', '2c', '-c', 'monitoring',
        '-t', '5', '-r', '1', '-Oqn',
        '192.168.200.6', '1.3.6.1.2.1.2.2.1.2',
    ]);
});

it('reads oid values from typed and quick snmp output', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $output = implode("\n", [
        '.1.3.6.1.2.1.1.1.0 STRING: MikroTik RouterOS 7.15.3 (stable)',
        '.1.3.6.1.2.1.1.3.0 Timeticks: (123456789) 14 days, 6:56:07.89',
        '.1.3.6.1.2.1.2.2.1.8.1 INTEGER: 1',
        '.1.3.6.1.2.1.31.1.1.1.15.1 1000',
        '',
    ]);

    expect($adapter->parseSnmpLines($output))->toBe([
        '1.3.6.1.2.1.1.1.0' => 'MikroTik RouterOS 7.15.3 (stable)',
        '1.3.6.1.2.1.1.3.0' => '(123456789) 14 days, 6:56:07.89',
        '1.3.6.1.2.1.2.2.1.8.1' => '1',
        '1.3.6.1.2.1.31.1.1.1.15.1' => '1000',
    ])->and($adapter->snmpUptimeSeconds('(123456789) 14 days, 6:56:07.89'))->toBe(1234567)
        ->and($adapter->snmpUptimeSeconds('123456789'))->toBe(1234567)
        ->and($adapter->snmpUptimeSeconds(null))->toBeNull();
});

it('turns two counter readings into a rate per interface', function () {
    $adapter = new NetworkSnmpProbeAdapter();

    // 1 MB in 10 s is 0.8 Mbps; 0.5 MB is 0.4 Mbps
    $rates = $adapter->counterDeltas(
        ['1' => ['in' => 1_000_000, 'out' => 2_000_000, 'at' => 1000.0]],
        ['1' => ['in' => 2_000_000, 'out' => 2_500_000]],
        1010.0,
    );

    expect($rates)->toBe(['1' => ['rx_mbps' => 0.8, 'tx_mbps' => 0.4]]);
});

it('returns a null rate when the counter reset or has no baseline', function () {
    $adapter = new NetworkSnmpProbeAdapter();

    $reset = $adapter->counterDeltas(
        ['1' => ['in' => 5_000_000, 'out' => 1, 'at' => 1000.0]],
        ['1' => ['in' => 10, 'out' => null]],
        1010.0,
    );

    $noBaseline = $adapter->counterDeltas([], ['1' => ['in' => 10, 'out' => 20]], 1010.0);

    expect($reset)->toBe(['1' => ['rx_mbps' => null, 'tx_mbps' => null]])
        ->and($noBaseline)->toBe(['1' => ['rx_mbps' => null, 'tx_mbps' => null]]);
});

it('parses SMTP reply codes from reply lines', function () {
    $adapter = new MailServerProbeAdapter();

    expect($adapter->smtpReplyCode('220 mail.kantor.co.id ESMTP ready'))->toBe(220);
    expect($adapter->smtpReplyCode('235 2.7.0 Authentication successful'))->toBe(235);
    expect($adapter->smtpReplyCode('334 VXNlcm5hbWU6'))->toBe(334);
    expect($adapter->smtpReplyCode('535 5.7.8 Authentication credentials invalid'))->toBe(535);
});

it('detects OK and non-OK tagged IMAP replies', function () {
    $adapter = new MailServerProbeAdapter();

    expect($adapter->imapReplyIsOk('a001 OK LOGIN completed'))->toBeTrue();
    expect($adapter->imapReplyIsOk('a001 NO [AUTHENTICATIONFAILED] authentication failed'))->toBeFalse();
    expect($adapter->imapReplyIsOk('a002 OK LOGOUT completed', 'a002'))->toBeTrue();
});

it('mailServicePort defaults to smtp on 587', function () {
    $adapter = (new MailServerProbeAdapter())->configure(['host' => 'm']);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 587, 'encryption' => 'starttls']);
});

it('mailServicePort prefers the shared port column with imap mode', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'mail_check_mode' => 'imap', 'port' => 1993]);

    expect($adapter->mailServicePort())->toBe(['mode' => 'imap', 'port' => 1993, 'encryption' => 'starttls']);
});

it('mailServicePort falls back to legacy check_ports', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'check_ports' => ['smtp' => 2525, 'imap' => 993]]);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 2525, 'encryption' => 'starttls']);
});

it('mailServicePort falls back to legacy single-port keys', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'mail_check_mode' => 'smtp', 'smtp_port' => 2526]);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 2526, 'encryption' => 'starttls']);
});

it('mailServicePort forces an unknown mode back to smtp', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'mail_check_mode' => 'pop3']);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 587, 'encryption' => 'starttls']);
});

it('mailServicePort reads implicit TLS from the smtp port 465', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'port' => 465]);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 465, 'encryption' => 'ssl']);
});

it('mailServicePort reads implicit TLS from the imap port 993', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'mail_check_mode' => 'imap', 'port' => 993]);

    expect($adapter->mailServicePort())->toBe(['mode' => 'imap', 'port' => 993, 'encryption' => 'ssl']);
});

it('mailServicePort keeps an explicit encryption choice over the port default', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'port' => 587, 'mail_encryption' => 'none']);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 587, 'encryption' => 'none']);
});

it('mailServicePort falls back to auto for an unknown encryption choice', function () {
    $adapter = (new MailServerProbeAdapter())
        ->configure(['host' => 'm', 'port' => 587, 'mail_encryption' => 'sslv3']);

    expect($adapter->mailServicePort())->toBe(['mode' => 'smtp', 'port' => 587, 'encryption' => 'starttls']);
});

it('greets with the target host over EHLO', function () {
    $pair = @stream_socket_pair(STREAM_PF_INET, STREAM_SOCK_STREAM, STREAM_IPPROTO_IP);
    if ($pair === false) {
        $this->markTestSkipped('stream_socket_pair unavailable on this runner');
    }

    [$server, $client] = $pair;
    fwrite($server, "250 mail.example.test greets mail.example.test\r\n");

    $adapter = new MailServerProbeAdapter();
    $reply = $adapter->smtpEhloReply($client, 'mail.example.test');

    expect($adapter->smtpReplyCode((string) $reply))->toBe(250);
    expect(trim((string) fgets($server)))->toBe('EHLO mail.example.test');
    stream_set_blocking($server, false);
    expect(fgets($server))->toBeFalse();
    fclose($server);
    fclose($client);
});

it('falls back to HELO when EHLO is rejected', function () {
    $pair = @stream_socket_pair(STREAM_PF_INET, STREAM_SOCK_STREAM, STREAM_IPPROTO_IP);
    if ($pair === false) {
        $this->markTestSkipped('stream_socket_pair unavailable on this runner');
    }

    [$server, $client] = $pair;
    fwrite($server, "550 rejected\r\n250 mail.example.test HELO\r\n");

    $adapter = new MailServerProbeAdapter();
    $reply = $adapter->smtpEhloReply($client, 'mail.example.test');

    expect($reply)->not->toBeNull();
    expect($adapter->smtpReplyCode($reply))->toBe(250);
    expect(trim((string) fgets($server)))->toBe('EHLO mail.example.test');
    expect(trim((string) fgets($server)))->toBe('HELO mail.example.test');
    fclose($server);
    fclose($client);
});

it('reads a multiline SMTP greeting as one reply', function () {
    $pair = @stream_socket_pair(STREAM_PF_INET, STREAM_SOCK_STREAM, STREAM_IPPROTO_IP);
    if ($pair === false) {
        $this->markTestSkipped('stream_socket_pair unavailable on this runner');
    }

    [$server, $client] = $pair;
    fwrite($server, "220-mail.example.test ESMTP\r\n220 ready to talk\r\n");

    $reply = mailProbeCall(new MailServerProbeAdapter(), 'smtpReadReply', [$client]);

    expect($reply)->toBe("220-mail.example.test ESMTP\r\n220 ready to talk\r\n");
    fclose($server);
    fclose($client);
});

it('lists AUTH mechanisms from an EHLO reply and none when AUTH is absent', function () {
    $adapter = new MailServerProbeAdapter();

    // No AUTH line: the probe must report "AUTH not advertised" instead of
    // reading the leftover extension lines as an AUTH reply.
    expect(mailProbeCall($adapter, 'smtpAuthMechanisms', ["250-mail.example.test\r\n250-STARTTLS\r\n250 SIZE 10240000\r\n"]))
        ->toBe([]);

    expect(mailProbeCall($adapter, 'smtpAuthMechanisms', ["250-mail.example.test\r\n250-AUTH LOGIN PLAIN\r\n250 SIZE 10240000\r\n"]))
        ->toBe(['LOGIN', 'PLAIN']);

    expect(mailProbeCall($adapter, 'smtpAuthMechanisms', ["250 AUTH=LOGIN\r\n"]))
        ->toBe(['LOGIN']);
});

it('accepts TLS 1.3 in the probe crypto methods', function () {
    $methods = (new ReflectionClass(MailServerProbeAdapter::class))->getReflectionConstant('TLS_METHODS')->getValue();

    expect($methods & STREAM_CRYPTO_METHOD_TLSv1_3_CLIENT)->not->toBe(0)
        ->and($methods & STREAM_CRYPTO_METHOD_TLS_CLIENT)->not->toBe(0);
});

it('maps a successful spatie ping result to stats', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $result = PingResult::fromArray([
        'success' => true,
        'error' => null,
        'host' => '10.0.0.1',
        'packet_loss_percentage' => 0,
        'packets_transmitted' => 2,
        'packets_received' => 2,
        'timings' => [
            'minimum_time_in_ms' => 1.1,
            'maximum_time_in_ms' => 3.9,
            'average_time_in_ms' => 2.5,
            'standard_deviation_time_in_ms' => 0.8,
        ],
    ]);

    expect($adapter->pingResultToStats($result))->toBe([
        'success' => true,
        'avg_ms' => 2.5,
        'packet_loss' => 0.0,
        'jitter' => 0.8,
        'error' => null,
        'raw' => null,
    ]);
});

it('maps a failed spatie ping result to stats', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $result = PingResult::fromArray([
        'success' => false,
        'error' => 'timeout',
        'host' => '10.0.0.1',
        'packet_loss_percentage' => 100,
    ]);

    expect($adapter->pingResultToStats($result))->toBe([
        'success' => false,
        'avg_ms' => null,
        'packet_loss' => 100.0,
        'jitter' => null,
        'error' => 'timeout',
        'raw' => null,
    ]);
});

it('keeps ping stdout in the failure stats', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $result = PingResult::fromArray([
        'success' => false,
        'error' => 'unknownError',
        'host' => 'bad-host.invalid',
        'packet_loss_percentage' => 100,
        'raw_output' => "Ping request could not find host bad-host.invalid.\r\n",
    ]);

    expect($adapter->pingResultToStats($result))->toBe([
        'success' => false,
        'avg_ms' => null,
        'packet_loss' => 100.0,
        'jitter' => null,
        'error' => 'unknownError',
        'raw' => 'Ping request could not find host bad-host.invalid.',
    ]);
});

it('builds ping arguments for the current OS', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $arguments = $adapter->pingCommandArguments('1.1.1.1', 2);

    if (PHP_OS_FAMILY === 'Windows') {
        // ping.exe takes -n (count) and -w (timeout in ms); -c/-W are Linux flags
        expect($arguments)->toBe(['ping', '-n', '2', '-w', '5000', '1.1.1.1']);
    } else {
        expect($arguments)->toBeNull();
    }
});

it('parses windows ping statistics', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $output = implode("\n", [
        'Pinging 10.10.0.5 with 32 bytes of data:',
        '',
        'Reply from 10.10.0.5: bytes=32 time=1ms TTL=128',
        'Reply from 10.10.0.5: bytes=32 time=1ms TTL=128',
        'Request timed out.',
        'Reply from 10.10.0.5: bytes=32 time=2ms TTL=128',
        '',
        'Ping statistics for 10.10.0.5:',
        '    Packets: Sent = 4, Received = 3, Lost = 1 (25% loss),',
        'Approximate round trip times in milli-seconds:',
        '    Minimum = 1ms, Maximum = 2ms, Average = 1ms',
    ]);
    $result = PingResult::fromArray([
        'success' => true,
        'error' => null,
        'host' => '10.10.0.5',
        'packet_loss_percentage' => 0,
        'raw_output' => $output,
    ]);

    expect($adapter->windowsAverageTimeInMs($result))->toBe(1.0)
        ->and($adapter->windowsPacketLossPercentage($result))->toBe(25.0);
});

it('returns null for statistics that are not in the windows format', function () {
    $adapter = new NetworkSnmpProbeAdapter();
    $result = PingResult::fromArray([
        'success' => false,
        'error' => 'unknownError',
        'host' => 'bad-host.invalid',
        'raw_output' => 'Ping request could not find host bad-host.invalid.',
    ]);

    expect($adapter->windowsAverageTimeInMs($result))->toBeNull()
        ->and($adapter->windowsPacketLossPercentage($result))->toBeNull()
        ->and($adapter->pingFailureDetail($result))->toBe('Ping request could not find host bad-host.invalid.');
});

it('classifies a windows ping failure from its own wording', function () {
    $adapter = new NetworkSnmpProbeAdapter();

    $notFound = PingResult::fromArray([
        'success' => false,
        'error' => 'unknownError',
        'host' => 'bad-host.invalid',
        'raw_output' => "Ping request could not find host bad-host.invalid.",
    ]);
    $timedOut = PingResult::fromArray([
        'success' => false,
        'error' => 'unknownError',
        'host' => '10.255.255.1',
        'raw_output' => "Request timed out.",
    ]);
    $linuxStyle = PingResult::fromArray([
        'success' => false,
        'error' => 'unknownError',
        'host' => '10.0.0.1',
        'raw_output' => 'some unrelated failure text',
    ]);

    expect($adapter->windowsPingError($notFound))->toBe('host not found')
        ->and($adapter->windowsPingError($timedOut))->toBe('request timed out')
        ->and($adapter->windowsPingError($linuxStyle))->toBeNull();
});
