<?php

namespace App\Support;

use Illuminate\Database\Eloquent\Concerns\HasUuids;

/**
 * UUID primary key trait for core entities.
 * Wraps Laravel's built-in HasUuids so every model gets time-ordered UUID v7 keys.
 */
trait HasUuid
{
    use HasUuids;

    /**
     * Disable auto-incrementing and set key type to string.
     */
    public function getIncrementing(): bool
    {
        return false;
    }

    public function getKeyType(): string
    {
        return 'string';
    }
}
