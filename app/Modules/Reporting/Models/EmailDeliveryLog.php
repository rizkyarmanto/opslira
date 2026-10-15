<?php

namespace App\Modules\Reporting\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class EmailDeliveryLog extends Model
{
    use HasUuid;

    protected $table = 'email_delivery_logs';

    public $timestamps = false;

    protected $guarded = [];

    protected function casts(): array
    {
        return [
            'retry_count' => 'integer',
            'sent_at' => 'datetime',
            'created_at' => 'datetime',
        ];
    }

    public function generatedReport(): BelongsTo
    {
        return $this->belongsTo(GeneratedReport::class, 'generated_report_id');
    }

    public function incident(): BelongsTo
    {
        return $this->belongsTo(\App\Modules\Incidents\Models\Incident::class, 'incident_id');
    }
}
