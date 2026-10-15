<?php

namespace App\Modules\Incidents\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Incident extends Model
{
    use HasUuid;

    protected $table = 'incidents';

    public $timestamps = false;

    protected $guarded = [];

    protected function casts(): array
    {
        return [
            'started_at' => 'datetime',
            'resolved_at' => 'datetime',
            'duration_seconds' => 'integer',
            'created_at' => 'datetime',
        ];
    }

    public function target(): BelongsTo
    {
        return $this->belongsTo(\App\Modules\Targets\Models\MonitoringTarget::class, 'target_id');
    }

    public function emailDeliveryLogs(): HasMany
    {
        return $this->hasMany(\App\Modules\Reporting\Models\EmailDeliveryLog::class, 'incident_id');
    }

    public function scopeOpen($query)
    {
        return $query->where('status', '!=', 'RESOLVED');
    }
}
