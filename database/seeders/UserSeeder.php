<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\Hash;

class UserSeeder extends Seeder
{
    public function run(): void
    {
        $users = [
            ['name' => 'Opslira Super Admin', 'email' => 'admin@opslira.local', 'role' => 'SUPER_ADMIN', 'password' => 'ChangeMe!2026'],
            ['name' => 'Network SysAdmin', 'email' => 'sysadmin@opslira.local', 'role' => 'SYSADMIN', 'password' => 'ChangeMe!2026'],
            ['name' => 'Application Developer', 'email' => 'developer@opslira.local', 'role' => 'DEVELOPER', 'password' => 'ChangeMe!2026'],
            ['name' => 'IT Manager', 'email' => 'manager@opslira.local', 'role' => 'MANAGER', 'password' => 'ChangeMe!2026'],
        ];

        foreach ($users as $user) {
            User::updateOrCreate(
                ['email' => $user['email']],
                [
                    'name' => $user['name'],
                    'role' => $user['role'],
                    'is_active' => true,
                    'password_hash' => Hash::make($user['password']),
                ],
            );
        }
    }
}
