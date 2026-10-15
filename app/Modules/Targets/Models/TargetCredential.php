<?php

namespace App\Modules\Targets\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class TargetCredential extends Model
{
    use HasUuid;

    protected $table = 'target_credentials';

    public $timestamps = false;

    protected $fillable = [
        'target_id',
        'auth_type',
        'encrypted_payload',
        'encryption_iv',
        'encryption_tag',
        'updated_at',
    ];

    protected function casts(): array
    {
        return [
            'updated_at' => 'datetime',
        ];
    }

    public function target(): BelongsTo
    {
        return $this->belongsTo(MonitoringTarget::class, 'target_id');
    }
}
