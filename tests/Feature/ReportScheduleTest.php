<?php

use App\Models\User;
use App\Modules\Reporting\Models\GeneratedReport;
use App\Modules\Reporting\Models\ReportSchedule;
use Illuminate\Foundation\Testing\DatabaseTransactions;

uses(DatabaseTransactions::class);

function schedulePayload(array $overrides = []): array
{
    return array_merge([
        'name' => 'Morning NOC summary',
        'period_type' => 'CUSTOM_CRON',
        'cron_expression' => '*/5 * * * *',
        'report_formats' => ['HTML'],
        'recipients_to' => ['noc@kantor.co.id'],
    ], $overrides);
}

it('rejects an invalid cron expression', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->postJson('/api/v1/reports/schedules', schedulePayload([
            'cron_expression' => 'not a cron',
        ]))
        ->assertStatus(422)
        ->assertJsonValidationErrors(['cron_expression']);
});

it('computes next_run_at from the cron expression on create', function () {
    $user = User::factory()->create();

    $response = $this->actingAs($user)
        ->postJson('/api/v1/reports/schedules', schedulePayload());

    $response->assertCreated();

    $nextRun = \Illuminate\Support\Carbon::parse($response->json('schedule.next_run_at'));
    expect($nextRun->greaterThan(\Illuminate\Support\Carbon::now()))
        ->toBeTrue()
        ->and($nextRun->lessThan(\Illuminate\Support\Carbon::now()->addMinutes(6)))
        ->toBeTrue();
});

it('recomputes next_run_at when the cron expression changes', function () {
    $user = User::factory()->create();
    $created = $this->actingAs($user)
        ->postJson('/api/v1/reports/schedules', schedulePayload())
        ->assertCreated();
    $id = $created->json('schedule.id');

    $this->actingAs($user)
        ->putJson("/api/v1/reports/schedules/{$id}", ['cron_expression' => '30 2 * * *'])
        ->assertOk();

    $schedule = ReportSchedule::findOrFail($id);
    expect($schedule->next_run_at->greaterThan(\Illuminate\Support\Carbon::now()->addMinutes(6)))
        ->toBeTrue();
});

it('does not move next_run_at when only the name changes', function () {
    $user = User::factory()->create();
    $created = $this->actingAs($user)
        ->postJson('/api/v1/reports/schedules', schedulePayload())
        ->assertCreated();
    $id = $created->json('schedule.id');
    $originalNextRun = \Illuminate\Support\Carbon::parse($created->json('schedule.next_run_at'));

    $this->actingAs($user)
        ->putJson("/api/v1/reports/schedules/{$id}", ['name' => 'Renamed schedule'])
        ->assertOk();

    $moved = \Illuminate\Support\Carbon::parse(ReportSchedule::findOrFail($id)->next_run_at);
    expect($moved->equalTo($originalNextRun))->toBeTrue();
});

it('hard deletes the schedule and keeps generated report history', function () {
    $user = User::factory()->create();
    $created = $this->actingAs($user)
        ->postJson('/api/v1/reports/schedules', schedulePayload())
        ->assertCreated();
    $id = $created->json('schedule.id');

    $report = GeneratedReport::create([
        'schedule_id' => $id,
        'trigger_source' => 'SCHEDULED',
        'period_type' => 'DAILY',
        'period_start' => now()->subDay(),
        'period_end' => now(),
        'generation_status' => 'COMPLETED',
    ]);

    $this->actingAs($user)
        ->deleteJson("/api/v1/reports/schedules/{$id}")
        ->assertOk()
        ->assertJson(['ok' => true]);

    $this->assertDatabaseMissing('report_schedules', ['id' => $id]);

    // The FK is nullOnDelete: the report row survives with a cleared link.
    $report->refresh();
    expect($report->schedule_id)->toBeNull()
        ->and($this->assertDatabaseHas('generated_reports', ['id' => $report->id]));
});
