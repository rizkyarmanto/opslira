<?php

namespace App\Modules\Users\Controllers;

use App\Models\User;
use App\Modules\Users\Requests\StoreUserRequest;
use App\Modules\Users\Requests\UpdateUserRequest;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Hash;
use Illuminate\Validation\ValidationException;
use Inertia\Inertia;
use Inertia\Response;

class UserController
{
    public function index(Request $request): Response
    {
        return Inertia::render('Users/UserIndex', [
            'users' => User::query()
                ->orderBy('name')
                ->get(['id', 'name', 'email', 'role', 'is_active', 'created_at']),
            'canManageUsers' => true,
        ]);
    }

    public function store(StoreUserRequest $request): JsonResponse
    {
        $data = $request->validated();

        $user = User::create([
            'name' => $data['name'],
            'email' => $data['email'],
            'role' => $data['role'],
            'is_active' => $data['is_active'] ?? true,
            'password_hash' => Hash::make($data['password']),
        ]);

        return response()->json(['user' => $user], 201);
    }

    public function update(UpdateUserRequest $request, User $user): JsonResponse
    {
        $data = $request->validated();
        $isSelf = $user->id === $request->user()->id;

        if ($isSelf && ($data['role'] !== $user->role || ($data['is_active'] ?? true) === false)) {
            throw ValidationException::withMessages([
                'role' => 'You cannot change your own role or disable your own account.',
            ]);
        }

        $user->fill([
            'name' => $data['name'],
            'email' => $data['email'],
            'role' => $data['role'],
            'is_active' => $data['is_active'] ?? $user->is_active,
        ]);

        if (! empty($data['password'])) {
            $user->password_hash = Hash::make($data['password']);
        }

        $user->save();

        return response()->json(['user' => $user->fresh()]);
    }

    public function destroy(Request $request, User $user): JsonResponse
    {
        if ($user->id === $request->user()->id) {
            throw ValidationException::withMessages([
                'user' => 'You cannot disable your own account.',
            ]);
        }

        // Soft disable, never hard delete
        $user->update(['is_active' => false]);

        return response()->json(['ok' => true]);
    }
}
