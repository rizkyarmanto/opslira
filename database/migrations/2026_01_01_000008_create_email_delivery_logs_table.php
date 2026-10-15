<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('email_delivery_logs', function (Blueprint $table) {
            $table->uuid('id')->primary();
            $table->string('email_category', 30);
            $table->foreignUuid('generated_report_id')->nullable()->constrained('generated_reports')->cascadeOnDelete();
            $table->foreignUuid('incident_id')->nullable()->constrained('incidents')->cascadeOnDelete();
            $table->string('recipient_email', 150);
            $table->string('subject', 255);
            $table->string('delivery_status', 20)->default('PENDING');
            $table->integer('retry_count')->default(0);
            $table->text('smtp_response_or_error')->nullable();
            $table->timestampTz('sent_at')->nullable();
            $table->timestampTz('created_at')->useCurrent();
        });

        DB::statement(
            "ALTER TABLE email_delivery_logs ADD CONSTRAINT edl_category_check
             CHECK (email_category IN ('SCHEDULED_REPORT','INCIDENT_ALERT','EXPIRY_WARNING'))"
        );
        DB::statement(
            "ALTER TABLE email_delivery_logs ADD CONSTRAINT edl_status_check
             CHECK (delivery_status IN ('PENDING','SENT','RETRYING','FAILED'))"
        );

        DB::statement('CREATE INDEX idx_email_logs_status_created ON email_delivery_logs (delivery_status, created_at DESC)');
    }

    public function down(): void
    {
        Schema::dropIfExists('email_delivery_logs');
    }
};
