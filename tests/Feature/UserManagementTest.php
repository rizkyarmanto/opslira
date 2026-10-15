<?php

use App\Models\User;
use Illuminate\Foundation\Testing\DatabaseTransactions;
use Illuminate\Support\Facades\Hash;

uses(DatabaseTransactions::class);

it('redirects a guest from the users page to login', function () {
    $this->get('/users')->assertRedirect('/login');
});

it('blocks a non-super-admin from the users page', function () {
    $user = User::factory()->create(['role' => 'DEVELOPER']);

    $this->actingAs($user)->get('/users')->assertForbidden();
});

it('lets a super admin open the users page', function () {
    $user = User::factory()->superAdmin()->create();

    $this->actingAs($user)->get('/users')->assertOk();
});

it('blocks a non-super-admin from the users api', function () {
    $user = User::factory()->create(['role' => 'DEVELOPER']);

    $this->actingAs($user)
        ->postJson('/api/v1/users', [
            'name' => 'Intruder',
            'email' => 'intruder@opslira.local',
            'role' => 'DEVELOPER',
            'password' => 'Secret!2026',
            'password_confirmation' => 'Secret!2026',
        ])
        ->assertForbidden();
});

it('creates a user as a super admin', function () {
    $admin = User::factory()->superAdmin()->create();

    $this->actingAs($admin)
        ->postJson('/api/v1/users', [
            'name' => 'New Operator',
            'email' => 'new.operator@opslira.local',
            'role' => 'SYSADMIN',
            'is_active' => true,
            'password' => 'Secret!2026',
            'password_confirmation' => 'Secret!2026',
        ])
        ->assertCreated();

    $this->assertDatabaseHas('users', [
        'email' => 'new.operator@opslira.local',
        'role' => 'SYSADMIN',
    ]);
});

it('rejects a duplicate email when creating a user', function () {
    $admin = User::factory()->superAdmin()->create();
    $existing = User::factory()->create(['email' => 'taken@opslira.local']);

    $this->actingAs($admin)
        ->postJson('/api/v1/users', [
            'name' => 'Duplicate',
            'email' => $existing->email,
            'role' => 'DEVELOPER',
            'password' => 'Secret!2026',
            'password_confirmation' => 'Secret!2026',
        ])
        ->assertStatus(422)
        ->assertJsonValidationErrors('email');
});

it('rejects a password under eight characters when creating a user', function () {
    $admin = User::factory()->superAdmin()->create();

    $this->actingAs($admin)
        ->postJson('/api/v1/users', [
            'name' => 'Short Password',
            'email' => 'short@opslira.local',
            'role' => 'DEVELOPER',
            'password' => 'short',
            'password_confirmation' => 'short',
        ])
        ->assertStatus(422)
        ->assertJsonValidationErrors('password');
});

it('updates another user role as a super admin', function () {
    $admin = User::factory()->superAdmin()->create();
    $target = User::factory()->create(['role' => 'DEVELOPER']);

    $this->actingAs($admin)
        ->putJson("/api/v1/users/{$target->id}", [
            'name' => $target->name,
            'email' => $target->email,
            'role' => 'MANAGER',
            'is_active' => true,
        ])
        ->assertOk();

    expect($target->fresh()->role)->toBe('MANAGER');
});

it('keeps the password when the update sends a blank one', function () {
    $admin = User::factory()->superAdmin()->create();
    $target = User::factory()->create();
    $originalHash = $target->password_hash;

    $this->actingAs($admin)
        ->putJson("/api/v1/users/{$target->id}", [
            'name' => $target->name,
            'email' => $target->email,
            'role' => $target->role,
            'is_active' => true,
            'password' => '',
            'password_confirmation' => '',
        ])
        ->assertOk();

    expect($target->fresh()->password_hash)->toBe($originalHash);
});

it('replaces the password when the update sends a new one', function () {
    $admin = User::factory()->superAdmin()->create();
    $target = User::factory()->create();
    $originalHash = $target->password_hash;

    $this->actingAs($admin)
        ->putJson("/api/v1/users/{$target->id}", [
            'name' => $target->name,
            'email' => $target->email,
            'role' => $target->role,
            'is_active' => true,
            'password' => 'BrandNew!2026',
            'password_confirmation' => 'BrandNew!2026',
        ])
        ->assertOk();

    $fresh = $target->fresh();
    expect($fresh->password_hash)->not->toBe($originalHash)
        ->and(Hash::check('BrandNew!2026', $fresh->password_hash))->toBeTrue();
});

it('refuses to let a super admin change their own role', function () {
    $admin = User::factory()->superAdmin()->create();

    $this->actingAs($admin)
        ->putJson("/api/v1/users/{$admin->id}", [
            'name' => $admin->name,
            'email' => $admin->email,
            'role' => 'MANAGER',
            'is_active' => true,
        ])
        ->assertStatus(422);

    expect($admin->fresh()->role)->toBe('SUPER_ADMIN');
});

it('refuses to let a super admin disable their own account', function () {
    $admin = User::factory()->superAdmin()->create();

    $this->actingAs($admin)
        ->deleteJson("/api/v1/users/{$admin->id}")
        ->assertStatus(422);

    expect($admin->fresh()->is_active)->toBeTrue();
});

it('disables another user without deleting the row', function () {
    $admin = User::factory()->superAdmin()->create();
    $target = User::factory()->create();

    $this->actingAs($admin)
        ->deleteJson("/api/v1/users/{$target->id}")
        ->assertOk();

    expect($target->fresh()->is_active)->toBeFalse();
    $this->assertDatabaseHas('users', ['id' => $target->id]);
});

it('signs out an inactive user who hits a protected page', function () {
    $user = User::factory()->inactive()->create();

    $this->actingAs($user)
        ->get('/dashboard/noc')
        ->assertRedirect('/login');

    $this->assertGuest();
});
