<?php

use App\Models\User;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Foundation\Testing\DatabaseTransactions;

uses(DatabaseTransactions::class);

function databaseTargetPayload(array $overrides = []): array
{
    return array_merge([
        'name' => 'Primary database',
        'dashboard_group' => 'APP_DOMAIN_DB',
        'target_type' => 'DATABASE',
        'environment' => 'PRODUCTION',
        'host_or_endpoint' => 'db-primary.example.internal',
        'protocol_type' => 'DB_NATIVE_PING',
        'check_interval_seconds' => 60,
        'timeout_seconds' => 5,
        'retry_threshold' => 3,
        'degraded_threshold_ms' => 1000,
        'config_payload' => [
            'db_driver' => 'pgsql',
            'database_name' => 'app',
            'query' => 'SELECT 1',
        ],
    ], $overrides);
}

it('rejects a database driver outside the supported list', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->postJson('/api/v1/targets', databaseTargetPayload([
            'config_payload' => ['db_driver' => 'postgres', 'database_name' => 'app'],
        ]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['config_payload.db_driver']);
});

it('stores a mariadb driver, which rows saved by the older form already use', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)->postJson('/api/v1/targets', databaseTargetPayload([
        'config_payload' => ['db_driver' => 'mariadb', 'database_name' => 'app', 'query' => 'SELECT 1'],
    ]));

    $response->assertCreated();

    $target = MonitoringTarget::findOrFail($response->json('target.id'));

    // The other config keys must survive the save: a rule keyed on
    // config_payload.<key> would drop the whole array from validated().
    expect($target->config_payload['db_driver'])->toBe('mariadb')
        ->and($target->config_payload['database_name'])->toBe('app')
        ->and($target->config_payload['query'])->toBe('SELECT 1');
});
