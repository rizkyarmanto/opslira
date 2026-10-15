<?php

namespace App\Modules\Probes\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * Monitoring log (partitioned table).
 * NOTE: Insert bulk via DB::table() for partition performance: this model is
 * for querying only (docs/gemini/dokumen_arsitektur_aplikasi_web.md:131-133).
 */
class MonitoringLog extends Model
{
    protected $table = 'monitoring_logs';

    public $timestamps = false;

    public $incrementing = true;

    protected $keyType = 'int';

    protected $guarded = [];

    protected function casts(): array
    {
        return [
            'metrics_payload' => 'array',
            'checked_at' => 'datetime',
            'response_time_ms' => 'float',
        ];
    }

    public function target(): BelongsTo
    {
        return $this->belongsTo(\App\Modules\Targets\Models\MonitoringTarget::class, 'target_id');
    }
}
