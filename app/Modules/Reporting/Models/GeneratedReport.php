<?php

namespace App\Modules\Reporting\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class GeneratedReport extends Model
{
    use HasUuid;

    protected $table = 'generated_reports';

    public $timestamps = false;

    protected $guarded = [];

    protected function casts(): array
    {
        return [
            'period_start' => 'datetime',
            'period_end' => 'datetime',
            'summary_snapshot' => 'array',
            'created_at' => 'datetime',
        ];
    }

    public function schedule(): BelongsTo
    {
        return $this->belongsTo(ReportSchedule::class, 'schedule_id');
    }

    public function emailDeliveryLogs(): HasMany
    {
        return $this->hasMany(EmailDeliveryLog::class, 'generated_report_id');
    }
}
