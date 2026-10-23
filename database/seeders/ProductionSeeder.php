<?php

namespace Database\Seeders;

use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\Hash;

class ProductionSeeder extends Seeder
{
    public function run(): void
    {
        $password = (string) env('ADMIN_PASSWORD', '');

        if ($password === '') {
            $this->command->error('ADMIN_PASSWORD is empty. Set it in the environment, then rerun the seeder.');

            return;
        }

        User::updateOrCreate(
            ['email' => (string) env('ADMIN_EMAIL', 'admin@opslira.local')],
            [
                'name' => (string) env('ADMIN_NAME', 'Opslira Super Admin'),
                'role' => User::ROLE_SUPER_ADMIN,
                'is_active' => true,
                'password_hash' => Hash::make($password),
            ],
        );

        $this->command->info('Production seed done: superadmin only.');
    }
}
