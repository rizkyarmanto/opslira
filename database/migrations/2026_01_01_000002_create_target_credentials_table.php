<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('target_credentials', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->foreignUuid('target_id')->unique()->constrained('monitoring_targets')->cascadeOnDelete();
            $table->string('auth_type', 30);
            $table->text('encrypted_payload');
            $table->string('encryption_iv', 64);
            $table->string('encryption_tag', 64);
            $table->timestampTz('updated_at')->useCurrent();
        });

        DB::statement(
            "ALTER TABLE target_credentials ADD CONSTRAINT tc_auth_type_check
             CHECK (auth_type IN ('NONE','DB_AUTH','SNMP_V2C','SNMP_V3','S3_KEY','BESZEL_TOKEN','HTTP_BEARER'))"
        );
    }

    public function down(): void
    {
        Schema::dropIfExists('target_credentials');
    }
};
