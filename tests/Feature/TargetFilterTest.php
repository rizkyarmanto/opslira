<?php

use App\Models\User;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Foundation\Testing\DatabaseTransactions;

uses(DatabaseTransactions::class);

/**
 * Seeds the three rows every assertion looks at. Assertions are written as
 * contains / not-contains on these names rather than row counts, because the
 * shared test database may already hold rows from other tests.
 */
function filterTargets(): void
{
    MonitoringTarget::factory()->create([
        'name' => 'Active Down Target',
        'current_status' => 'DOWN',
        'is_active' => true,
        'is_maintenance' => false,
    ]);
    MonitoringTarget::factory()->create([
        'name' => 'Disabled Target',
        'current_status' => 'UP',
        'is_active' => false,
        'is_maintenance' => false,
    ]);
    MonitoringTarget::factory()->create([
        'name' => 'Maintenance Target',
        'current_status' => 'MAINTENANCE',
        'is_active' => true,
        'is_maintenance' => true,
    ]);
}

/**
 * @return array<int, string>
 */
function listedTargetNames($response): array
{
    $response->assertOk()->assertInertia(fn ($page) => $page->component('Targets/TargetIndex'));

    return array_column($response->inertiaProps()['targets'], 'name');
}

it('defaults to active targets that are not in maintenance', function () {
    filterTargets();
    $user = User::factory()->create();

    $names = listedTargetNames($this->actingAs($user)->get('/targets'));

    expect($names)
        ->toContain('Active Down Target')
        ->not->toContain('Disabled Target')
        ->not->toContain('Maintenance Target');
});

it('returns only disabled targets with state=disabled', function () {
    filterTargets();
    $user = User::factory()->create();

    $response = $this->actingAs($user)->get('/targets?state=disabled');
    $names = listedTargetNames($response);

    expect($names)
        ->toContain('Disabled Target')
        ->not->toContain('Active Down Target')
        ->and($response->inertiaProps()['filters']['state'])->toBe('disabled');
});

it('returns only maintenance targets with maintenance=yes', function () {
    filterTargets();
    $user = User::factory()->create();

    $response = $this->actingAs($user)->get('/targets?maintenance=yes');
    $names = listedTargetNames($response);

    expect($names)
        ->toContain('Maintenance Target')
        ->not->toContain('Active Down Target')
        ->and($response->inertiaProps()['filters']['maintenance'])->toBe('yes');
});

it('filters by status', function () {
    filterTargets();
    $user = User::factory()->create();

    $names = listedTargetNames($this->actingAs($user)->get('/targets?status=DOWN'));

    expect($names)
        ->toContain('Active Down Target')
        ->not->toContain('Disabled Target')
        ->not->toContain('Maintenance Target');
});
