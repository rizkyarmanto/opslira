<?php

use App\Models\User;
use App\Modules\Targets\Models\TargetCredential;
use Illuminate\Support\Facades\Http;

function targetPayload(array $overrides = []): array
{
    return array_merge([
        'name' => 'Router Credential Test',
        'dashboard_group' => 'NETWORK',
        'target_type' => 'NETWORK_DEVICE',
        'environment' => 'INTERNAL',
        'host_or_endpoint' => '10.0.0.9',
        'protocol_type' => 'ICMP_SNMP',
        'config_payload' => ['sub_type' => 'ROUTER', 'icmp_count' => 4, 'enable_snmp' => false],
    ], $overrides);
}

it('stores encrypted SNMP v3 credentials and never returns them', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'credentials' => [
            'auth_type' => 'SNMP_V3',
            'payload' => [
                'username' => 'monuser',
                'auth_protocol' => 'SHA',
                'auth_password' => 'auth-secret-123',
                'priv_protocol' => 'AES',
                'priv_password' => 'priv-secret-456',
            ],
        ],
    ]));

    $response->assertCreated();
    $response->assertDontSee('auth-secret-123', false);
    $response->assertDontSee('priv-secret-456', false);

    $targetId = $response->json('target.id');
    $credential = TargetCredential::where('target_id', $targetId)->firstOrFail();

    expect($credential->auth_type)->toBe('SNMP_V3');
    expect($credential->encrypted_payload)->not->toContain('auth-secret-123');
});

it('rejects SNMP v3 without a priv passphrase', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'credentials' => [
            'auth_type' => 'SNMP_V3',
            'payload' => [
                'username' => 'monuser',
                'auth_protocol' => 'SHA',
                'auth_password' => 'auth-secret-123',
                'priv_protocol' => 'AES',
            ],
        ],
    ]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['credentials.payload.priv_password']);
});

it('rejects Beszel credentials with both an API token and a login', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'name' => 'Beszel Hub',
        'dashboard_group' => 'MAIL_S3_BESZEL',
        'target_type' => 'BESZEL_SERVER',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'http://beszel:8090',
        'protocol_type' => 'BESZEL_API',
        'config_payload' => [],
        'credentials' => [
            'auth_type' => 'BESZEL',
            'payload' => [
                'api_token' => 'tok_abc',
                'email' => 'ops@kantor.co.id',
                'password' => 'secret',
            ],
        ],
    ]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['credentials.payload']);
});

it('rejects Beszel credentials with neither a token nor a login', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'name' => 'Beszel Hub Empty',
        'dashboard_group' => 'MAIL_S3_BESZEL',
        'target_type' => 'BESZEL_SERVER',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'http://beszel:8091',
        'protocol_type' => 'BESZEL_API',
        'config_payload' => [],
        'credentials' => ['auth_type' => 'BESZEL', 'payload' => []],
    ]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['credentials.payload']);
});

it('stores HTTP basic credentials under the new auth type constraint', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'name' => 'Portal Health',
        'dashboard_group' => 'APP_DOMAIN_DB',
        'target_type' => 'WEB_APP',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'https://portal.kantor.co.id/health',
        'protocol_type' => 'HTTP_HEALTH',
        'config_payload' => ['method' => 'GET', 'expected_status' => 200],
        'credentials' => [
            'auth_type' => 'HTTP_BASIC',
            'payload' => ['username' => 'probe-user', 'password' => 'probe-pass'],
        ],
    ]));

    $response->assertCreated();

    $credential = TargetCredential::where('target_id', $response->json('target.id'))->firstOrFail();
    expect($credential->auth_type)->toBe('HTTP_BASIC');
});

it('sends Basic auth from test-connection credentials', function () {
    $user = User::factory()->create();

    Http::fake(['*' => Http::response(['status' => 'ok'], 200)]);

    $this->actingAs($user)->postJson('/api/v1/targets/test-connection', [
        'name' => 'Portal',
        'dashboard_group' => 'APP_DOMAIN_DB',
        'target_type' => 'WEB_APP',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'https://portal.kantor.co.id/health',
        'protocol_type' => 'HTTP_HEALTH',
        'config_payload' => ['url' => 'https://portal.kantor.co.id/health', 'method' => 'GET', 'expected_status' => 200, 'verify_ssl' => true],
        'credentials' => [
            'auth_type' => 'HTTP_BASIC',
            'payload' => ['username' => 'probe-user', 'password' => 'probe-pass'],
        ],
    ])->assertOk();

    Http::assertSent(fn ($request) => str_starts_with((string) $request->header('Authorization')[0], 'Basic '));
});

function mailPayload(array $credentials): array
{
    return [
        'name' => 'Mail Pair Target',
        'dashboard_group' => 'MAIL_S3_BESZEL',
        'target_type' => 'MAIL_SERVER',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'mail.ct.internal',
        'protocol_type' => 'SMTP_DNSBL',
        'config_payload' => ['check_ports' => ['smtp' => 587, 'imap' => 993], 'check_dnsbl' => false],
        'credentials' => ['auth_type' => 'MAIL_AUTH', 'payload' => $credentials],
    ];
}

it('stores MAIL credentials with only an IMAP pair', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', mailPayload([
        'imap_username' => 'imap-user',
        'imap_password' => 'imap-pass',
    ]));

    $response->assertCreated();

    $credential = TargetCredential::where('target_id', $response->json('target.id'))->firstOrFail();
    expect($credential->auth_type)->toBe('MAIL_AUTH');
});

it('stores MAIL credentials with only an SMTP pair', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', mailPayload([
        'smtp_username' => 'smtp-user',
        'smtp_password' => 'smtp-pass',
    ]));

    $response->assertCreated();

    $credential = TargetCredential::where('target_id', $response->json('target.id'))->firstOrFail();
    expect($credential->auth_type)->toBe('MAIL_AUTH');
});

it('rejects MAIL_AUTH without an SMTP/IMAP login pair', function () {
    $user = User::factory()->create();

    $this->actingAs($user)->postJson('/api/v1/targets', mailPayload([]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['credentials.payload']);
});

it('deletes stored credentials when an update sets auth_type to NONE', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', targetPayload([
        'name' => 'MinIO Bucket',
        'dashboard_group' => 'MAIL_S3_BESZEL',
        'target_type' => 'S3_STORAGE',
        'environment' => 'INTERNAL',
        'host_or_endpoint' => 'minio.kantor.co.id',
        'protocol_type' => 'S3_HEAD',
        'config_payload' => ['bucket_name' => 'backups', 'region' => 'us-east-1', 'path_style' => true],
        'credentials' => [
            'auth_type' => 'S3_KEY',
            'payload' => ['access_key' => 'AKIAEXAMPLE', 'secret_key' => 's3cret-key-value'],
        ],
    ]));
    $response->assertCreated();

    $targetId = $response->json('target.id');
    expect(TargetCredential::where('target_id', $targetId)->count())->toBe(1);

    $this->actingAs($user)->putJson("/api/v1/targets/{$targetId}", targetPayload([
        'name' => 'MinIO Bucket',
        'dashboard_group' => 'MAIL_S3_BESZEL',
        'target_type' => 'S3_STORAGE',
        'environment' => 'INTERNAL',
        'host_or_endpoint' => 'minio.kantor.co.id',
        'protocol_type' => 'S3_HEAD',
        'config_payload' => ['bucket_name' => 'backups', 'region' => 'us-east-1', 'path_style' => true],
        'credentials' => ['auth_type' => 'NONE', 'payload' => []],
    ]))->assertOk();

    expect(TargetCredential::where('target_id', $targetId)->count())->toBe(0);
});
