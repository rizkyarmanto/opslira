<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('incidents', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->foreignUuid('target_id')->constrained('monitoring_targets')->cascadeOnDelete();
            $table->string('severity', 20)->default('CRITICAL');
            $table->string('status', 20)->default('OPEN');
            $table->text('root_cause_message');
            $table->timestampTz('started_at')->useCurrent();
            $table->timestampTz('resolved_at')->nullable();
            $table->integer('duration_seconds')->nullable();
            $table->timestampTz('created_at')->useCurrent();
        });

        DB::statement(
            "ALTER TABLE incidents ADD CONSTRAINT incidents_severity_check
             CHECK (severity IN ('CRITICAL','WARNING'))"
        );
        DB::statement(
            "ALTER TABLE incidents ADD CONSTRAINT incidents_status_check
             CHECK (status IN ('OPEN','ACKNOWLEDGED','RESOLVED'))"
        );

        DB::statement('CREATE INDEX idx_incidents_target_started ON incidents (target_id, started_at DESC)');
        DB::statement("CREATE INDEX idx_incidents_open ON incidents (status) WHERE status != 'RESOLVED'");
    }

    public function down(): void
    {
        Schema::dropIfExists('incidents');
    }
};
