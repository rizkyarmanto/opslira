<?php

use App\Models\User;
use App\Modules\Targets\Models\MonitoringTarget;

it('redirects guests from the dashboard to login', function () {
    $this->get('/dashboard/noc')->assertRedirect('/login');
});

it('renders the NOC overview for authenticated users', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->get('/dashboard/noc')
        ->assertOk();
});

it('lists targets on the targets page', function () {
    $user = User::factory()->create();
    MonitoringTarget::factory()->create(['name' => 'Router Test']);

    $this->actingAs($user)
        ->get('/targets')
        ->assertOk();
});

it('creates a target via the API', function () {
    $user = User::factory()->create();

    $this->actingAs($user)
        ->postJson('/api/v1/targets', [
            'name' => 'New Router',
            'dashboard_group' => 'NETWORK',
            'target_type' => 'NETWORK_DEVICE',
            'environment' => 'INTERNAL',
            'host_or_endpoint' => '10.0.0.1',
            'protocol_type' => 'ICMP_SNMP',
            'config_payload' => ['sub_type' => 'ROUTER'],
        ])
        ->assertCreated();
});
