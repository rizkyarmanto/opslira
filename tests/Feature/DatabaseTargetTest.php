<?php

use App\Models\User;
use App\Modules\Targets\Models\MonitoringTarget;
use App\Modules\Targets\Services\DatabaseActivityService;
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

it('requires authentication to view database activity', function () {
    $target = MonitoringTarget::create(databaseTargetPayload());
    $this->getJson("/api/v1/targets/{$target->id}/database-activity")->assertUnauthorized();
});

it('rejects unsupported targets for database activity', function () {
    $user = User::factory()->create();
    foreach ([
        ['target_type' => 'WEB_APP'],
        ['protocol_type' => 'HTTP_HEALTH'],
    ] as $overrides) {
        $target = MonitoringTarget::create(databaseTargetPayload($overrides));
        $this->actingAs($user)->getJson("/api/v1/targets/{$target->id}/database-activity")->assertNotFound();
    }
});

it('returns database activity snapshots without caching or changing values', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::create(databaseTargetPayload());
    $row = [
        'username' => 'monitor', 'database' => null, 'state' => 'idle',
        'state_duration' => '1d 01h 01m 01s', 'query_age' => null,
        'query_syntax' => "SELECT '<value>';\nSELECT 2;",
    ];
    $service = Mockery::mock(DatabaseActivityService::class);
    $service->shouldReceive('fetch')->twice()->andReturn([], [$row]);
    $this->app->instance(DatabaseActivityService::class, $service);

    $url = "/api/v1/targets/{$target->id}/database-activity";
    $this->actingAs($user)->getJson($url)->assertOk()->assertJsonPath('data', [])
        ->assertJsonStructure(['collected_at'])->assertHeader('Cache-Control', 'no-store, private');
    $this->getJson($url)->assertOk()->assertJsonPath('data', [$row]);
});

it('returns a safe error when database activity collection fails', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::create(databaseTargetPayload());
    $service = Mockery::mock(DatabaseActivityService::class);
    $service->shouldReceive('fetch')->once()->andThrow(new RuntimeException('secret SQL and credentials'));
    $this->app->instance(DatabaseActivityService::class, $service);

    $this->actingAs($user)->getJson("/api/v1/targets/{$target->id}/database-activity")
        ->assertStatus(503)
        ->assertExactJson(['message' => 'Could not load database activity. Check the connection and monitoring permissions.'])
        ->assertHeader('Cache-Control', 'no-store, private');
});
