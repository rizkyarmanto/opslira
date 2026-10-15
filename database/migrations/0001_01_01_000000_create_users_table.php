<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Enable pgcrypto for gen_random_uuid()
        DB::statement('CREATE EXTENSION IF NOT EXISTS "pgcrypto"');

        Schema::create('users', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->string('name', 120);
            $table->string('email', 150)->unique();
            $table->string('password_hash', 255);
            $table->string('role', 30)->default('DEVELOPER');
            $table->boolean('is_active')->default(true);
            $table->timestampsTz();
        });

        // CHECK constraint for role (Laravel builder has no native check support)
        DB::statement(
            "ALTER TABLE users ADD CONSTRAINT users_role_check
             CHECK (role IN ('SUPER_ADMIN', 'SYSADMIN', 'DEVELOPER', 'MANAGER'))"
        );

        Schema::create('password_reset_tokens', function (Blueprint $table) {
            $table->string('email')->primary();
            $table->string('token');
            $table->timestampTz('created_at')->nullable();
        });

        Schema::create('sessions', function (Blueprint $table) {
            $table->string('id')->primary();
            $table->foreignUuid('user_id')->nullable()->index();
            $table->string('ip_address', 45)->nullable();
            $table->text('user_agent')->nullable();
            $table->longText('payload');
            $table->integer('last_activity')->index();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('sessions');
        Schema::dropIfExists('password_reset_tokens');
        Schema::dropIfExists('users');
    }
};
