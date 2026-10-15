<?php

namespace App\Modules\Reporting\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class ReportSchedule extends Model
{
    use HasUuid;

    protected $table = 'report_schedules';

    protected $fillable = [
        'name',
        'period_type',
        'cron_expression',
        'timezone',
        'scope_filter',
        'report_formats',
        'recipients_to',
        'recipients_cc',
        'send_only_on_issues',
        'is_active',
        'last_run_at',
        'next_run_at',
        'created_by',
    ];

    protected function casts(): array
    {
        return [
            'scope_filter' => 'array',
            'report_formats' => 'array',
            'recipients_to' => 'array',
            'recipients_cc' => 'array',
            'send_only_on_issues' => 'boolean',
            'is_active' => 'boolean',
            'last_run_at' => 'datetime',
            'next_run_at' => 'datetime',
        ];
    }

    public function generatedReports(): HasMany
    {
        return $this->hasMany(GeneratedReport::class, 'schedule_id');
    }

    public function scopeDue($query)
    {
        return $query->where('is_active', true)->where('next_run_at', '<=', now());
    }
}
