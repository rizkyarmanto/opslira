<?php

use App\Models\User;
use App\Modules\Incidents\Models\Incident;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Foundation\Http\Middleware\ValidateCsrfToken;
use Illuminate\Foundation\Testing\DatabaseTransactions;
use Illuminate\Support\Str;

uses(DatabaseTransactions::class);

/**
 * CSRF is dropped for the POST calls on purpose: the middleware's own
 * test-time bypass depends on APP_ENV resolving to `testing`, which the
 * environment this project runs in does not guarantee, so the assertions here
 * would otherwise measure token plumbing instead of the resolve endpoint.
 */

function openIncident(MonitoringTarget $target, int $minutesAgo): Incident
{
    return Incident::create([
        'target_id' => $target->id,
        'severity' => 'CRITICAL',
        'status' => 'OPEN',
        'root_cause_message' => 'Probe timeout',
        'started_at' => now()->subMinutes($minutesAgo),
    ]);
}

it('resolves an incident with a positive integer duration', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::factory()->create();
    $incident = openIncident($target, 5);

    $this->actingAs($user)
        ->withoutMiddleware(ValidateCsrfToken::class)
        ->postJson("/api/v1/incidents/{$incident->id}/resolve")
        ->assertOk()
        ->assertJsonPath('incident.status', 'RESOLVED')
        ->assertJsonPath('incident.target_id', $target->id);

    $fresh = $incident->fresh();

    // The column is an integer and the probe path once wrote a signed float, so
    // the type and the sign are asserted, not only that a value is present.
    expect($fresh->status)->toBe('RESOLVED')
        ->and($fresh->resolved_at)->not->toBeNull()
        ->and($fresh->duration_seconds)->toBeInt()
        ->and($fresh->duration_seconds)->toBeGreaterThanOrEqual(300)
        ->and($fresh->duration_seconds)->toBeLessThan(600);
});

it('leaves a resolved incident untouched on a second call', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::factory()->create();
    $incident = openIncident($target, 5);

    $first = $this->actingAs($user)
        ->withoutMiddleware(ValidateCsrfToken::class)
        ->postJson("/api/v1/incidents/{$incident->id}/resolve")
        ->assertOk();

    $resolvedAt = $incident->fresh()->resolved_at->getTimestamp();
    $duration = $incident->fresh()->duration_seconds;

    $second = $this->actingAs($user)
        ->withoutMiddleware(ValidateCsrfToken::class)
        ->postJson("/api/v1/incidents/{$incident->id}/resolve")
        ->assertOk();

    $after = $incident->fresh();

    expect($after->resolved_at->getTimestamp())->toBe($resolvedAt)
        ->and($after->duration_seconds)->toBe($duration)
        ->and($second->json('incident.id'))->toBe($first->json('incident.id'));
});

it('answers 404 for an unknown incident id', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->withoutMiddleware(ValidateCsrfToken::class)
        ->postJson('/api/v1/incidents/'.Str::uuid().'/resolve')
        ->assertNotFound();
});

it('rejects an unauthenticated resolve', function () {
    $target = MonitoringTarget::factory()->create();
    $incident = openIncident($target, 5);

    $this->withoutMiddleware(ValidateCsrfToken::class)
        ->postJson("/api/v1/incidents/{$incident->id}/resolve")
        ->assertUnauthorized();
});

it('lists a target history newest first with resolved rows kept', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::factory()->create();

    $older = openIncident($target, 30);
    $older->update([
        'status' => 'RESOLVED',
        'resolved_at' => now()->subMinutes(25),
        'duration_seconds' => 300,
    ]);
    $newer = openIncident($target, 5);

    $response = $this->actingAs($user)->getJson("/api/v1/targets/{$target->id}/incidents");

    $response->assertOk();

    expect(array_column($response->json('data'), 'id'))->toBe([$newer->id, $older->id])
        ->and(array_column($response->json('data'), 'status'))->toBe(['OPEN', 'RESOLVED']);
});
