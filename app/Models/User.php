<?php

namespace App\Models;

use App\Support\HasUuid;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;

class User extends Authenticatable
{
    use HasFactory, HasUuid, Notifiable;

    public const ROLE_SUPER_ADMIN = 'SUPER_ADMIN';

    public const ROLES = ['SUPER_ADMIN', 'SYSADMIN', 'DEVELOPER', 'MANAGER'];

    protected $fillable = [
        'name',
        'email',
        'password_hash',
        'role',
        'is_active',
    ];

    protected $hidden = [
        'password_hash',
    ];

    protected function casts(): array
    {
        return [
            'is_active' => 'boolean',
            'created_at' => 'datetime',
            'updated_at' => 'datetime',
        ];
    }

    /**
     * Laravel expects a `password` attribute; our column is `password_hash`.
     */
    public function getAuthPassword(): string
    {
        return $this->password_hash;
    }

    public function isSuperAdmin(): bool
    {
        return $this->role === self::ROLE_SUPER_ADMIN;
    }
}
