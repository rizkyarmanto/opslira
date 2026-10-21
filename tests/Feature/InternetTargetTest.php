<?php

use App\Models\User;
use App\Modules\Probes\Adapters\BaseProbeAdapter;
use App\Modules\Probes\Contracts\ProbeAdapterInterface;
use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeAdapterFactory;

function internetPayload(array $overrides = []): array
{
    return array_merge([
        'name' => 'CT Internet Check',
        'dashboard_group' => 'NETWORK',
        'target_type' => 'INTERNET',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => '8.8.8.8',
        'protocol_type' => 'ICMP_PING',
        'config_payload' => ['icmp_count' => 4],
        'timeout_seconds' => 10,
    ], $overrides);
}

it('stores an internet target as ping only', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', internetPayload());

    $response->assertCreated();
    expect($response->json('target.protocol_type'))->toBe('ICMP_PING')
        ->and($response->json('target.config_payload.icmp_only'))->toBeTrue()
        ->and($response->json('target.dashboard_group'))->toBe('NETWORK');
});

it('forces the ping protocol when a client submits another one for an internet target', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', internetPayload([
        'protocol_type' => 'HTTP_HEALTH',
    ]));

    $response->assertCreated();
    expect($response->json('target.protocol_type'))->toBe('ICMP_PING');
});

it('drops the ping only flag when a target stops being an internet target', function () {
    $user = User::factory()->create();
    $created = $this->actingAs($user)->postJson('/api/v1/targets', internetPayload())->assertCreated();
    $targetId = $created->json('target.id');

    $updated = $this->actingAs($user)->putJson("/api/v1/targets/{$targetId}", internetPayload([
        'name' => 'CT Reassigned Target',
        'target_type' => 'NETWORK_DEVICE',
        'protocol_type' => 'ICMP_SNMP',
        'config_payload' => ['sub_type' => 'ROUTER', 'icmp_count' => 4, 'enable_snmp' => false],
    ]));

    $updated->assertOk();
    expect($updated->json('target.target_type'))->toBe('NETWORK_DEVICE')
        ->and($updated->json('target.protocol_type'))->toBe('ICMP_SNMP')
        ->and($updated->json('target.config_payload.icmp_only'))->toBeNull();
});

it('applies the submitted timeout to the pre save test connection', function () {
    $user = User::factory()->create();

    $probe = new class extends BaseProbeAdapter
    {
        public function supportsProtocol(string $protocolType): bool
        {
            return true;
        }

        public function execute(): ProbeResult
        {
            return ProbeResult::up(4.0);
        }

        public function observedTimeout(): int
        {
            return $this->timeoutSeconds;
        }
    };

    $factory = new class($probe) extends ProbeAdapterFactory
    {
        public function __construct(private readonly ProbeAdapterInterface $probe)
        {
        }

        public function make(string $protocolType): ProbeAdapterInterface
        {
            return $this->probe;
        }
    };
    $this->app->instance(ProbeAdapterFactory::class, $factory);

    $response = $this->actingAs($user)->postJson('/api/v1/targets/test-connection', internetPayload([
        'timeout_seconds' => 30,
    ]));

    $response->assertOk();
    expect($probe->observedTimeout())->toBe(30);
});
