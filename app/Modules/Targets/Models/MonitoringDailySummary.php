<?php

namespace App\Modules\Targets\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class MonitoringDailySummary extends Model
{
    protected $table = 'monitoring_daily_summaries';

    public $timestamps = false;

    protected $guarded = [];

    protected function casts(): array
    {
        return [
            'summary_date' => 'date',
            'daily_metrics_snapshot' => 'array',
            'uptime_percentage' => 'float',
            'avg_response_time_ms' => 'float',
            'max_response_time_ms' => 'float',
            'p95_response_time_ms' => 'float',
            'created_at' => 'datetime',
        ];
    }

    public function target(): BelongsTo
    {
        return $this->belongsTo(MonitoringTarget::class, 'target_id');
    }
}
