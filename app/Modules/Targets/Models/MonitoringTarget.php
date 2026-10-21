<?php

namespace App\Modules\Targets\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\HasOne;

class MonitoringTarget extends Model
{
    use HasFactory, HasUuid;

    /**
     * The model lives in a nested namespace while its factory sits at the
     * top of Database\Factories; without this Laravel looks for a factory in
     * a mirrored subfolder and fails to resolve it.
     */
    protected static function newFactory(): \Database\Factories\MonitoringTargetFactory
    {
        return \Database\Factories\MonitoringTargetFactory::new();
    }

    protected $table = 'monitoring_targets';

    protected $fillable = [
        'name',
        'dashboard_group',
        'target_type',
        'environment',
        'host_or_endpoint',
        'port',
        'protocol_type',
        'config_payload',
        'check_interval_seconds',
        'timeout_seconds',
        'retry_threshold',
        'degraded_threshold_ms',
        'current_status',
        'is_maintenance',
        'is_active',
        'last_checked_at',
        'next_check_at',
        'created_by',
    ];

    protected function casts(): array
    {
        return [
            'config_payload' => 'array',
            'port' => 'integer',
            'check_interval_seconds' => 'integer',
            'timeout_seconds' => 'integer',
            'retry_threshold' => 'integer',
            'degraded_threshold_ms' => 'integer',
            'is_maintenance' => 'boolean',
            'is_active' => 'boolean',
            'last_checked_at' => 'datetime',
            'next_check_at' => 'datetime',
        ];
    }

    public function credentials(): HasOne
    {
        return $this->hasOne(TargetCredential::class, 'target_id');
    }

    public function logs(): HasMany
    {
        return $this->hasMany(\App\Modules\Probes\Models\MonitoringLog::class, 'target_id');
    }

    public function dailySummaries(): HasMany
    {
        return $this->hasMany(MonitoringDailySummary::class, 'target_id');
    }

    public function incidents(): HasMany
    {
        return $this->hasMany(\App\Modules\Incidents\Models\Incident::class, 'target_id');
    }

    /**
     * Targets due for a probe check.
     */
    public function scopeDueForCheck($query)
    {
        return $query->where('is_active', true)
            ->where('is_maintenance', false)
            ->where('next_check_at', '<=', now());
    }

    public function scopeForDashboard($query, string $group)
    {
        return $query->where('dashboard_group', $group);
    }
}
