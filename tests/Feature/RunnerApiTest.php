<?php

use App\Modules\Reporting\Models\ReportSchedule;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Foundation\Testing\DatabaseTransactions;

uses(DatabaseTransactions::class);

const RUNNER_TOKEN = 'test-runner-token';

beforeEach(function () {
    config(['monitoring.runner_token' => RUNNER_TOKEN]);
});

function withRunnerToken($test)
{
    return $test->withHeaders(['X-Opslira-Runner' => RUNNER_TOKEN]);
}

it('rejects a runner request with the wrong token', function () {
    $this->withHeaders(['X-Opslira-Runner' => 'wrong-token'])
        ->postJson('/internal/probes/claim', ['limit' => 5])
        ->assertStatus(401)
        ->assertJson(['message' => 'Invalid runner token']);
});

it('rejects every internal route without a token', function () {
    $targets = MonitoringTarget::factory()->count(2)->create();

    foreach ([
        ['/internal/probes/claim', ['limit' => 5]],
        ['/internal/probes/'.$targets->first()->id, []],
        ['/internal/maintenance/rollup', []],
        ['/internal/maintenance/prune', []],
        ['/internal/reports/dispatch', []],
    ] as [$path, $payload]) {
        $this->postJson($path, $payload)->assertStatus(401);
        $this->withHeaders(['X-Opslira-Runner' => 'wrong'])
            ->postJson($path, $payload)
            ->assertStatus(401);
    }
});

it('claims due targets and advances next_check_at', function () {
    $due = MonitoringTarget::factory()->create(['next_check_at' => now()->subMinute()]);
    $dueNextRun = $due->next_check_at;
    $notDue = MonitoringTarget::factory()->create(['next_check_at' => now()->addHour()]);
    $notDueNextRun = $notDue->next_check_at;

    $response = withRunnerToken($this)
        ->postJson('/internal/probes/claim', ['limit' => 100]);

    $response->assertOk();
    $claimedIds = array_column($response->json('targets'), 'id');
    expect($claimedIds)->toContain($due->id)
        ->not->toContain($notDue->id);

    $due->refresh();
    $notDue->refresh();
    expect($due->next_check_at->greaterThan($dueNextRun))->toBeTrue()
        ->and($notDue->next_check_at->equalTo($notDueNextRun))->toBeTrue();
});

it('refuses to probe an inactive target', function () {
    $disabled = MonitoringTarget::factory()->create(['is_active' => false]);

    withRunnerToken($this)
        ->postJson('/internal/probes/'.$disabled->id)
        ->assertStatus(409)
        ->assertJson(['message' => 'Target is not due']);
});

it('returns an integer dispatched count', function () {
    // Deactivate any seeded schedule so the dispatch loop cannot trigger a real
    // report generation inside this test.
    ReportSchedule::query()->update(['is_active' => false]);

    $response = withRunnerToken($this)->postJson('/internal/reports/dispatch');

    $response->assertOk();
    expect($response->json('dispatched'))->toBeInt();
});
