<?php

namespace App\Modules\Targets\Requests;

use Illuminate\Foundation\Http\FormRequest;
use Illuminate\Validation\Rule;

/**
 * Dynamic validation rules keyed by protocol_type / target_type.
 * Skema dinamis per protokol: docs/gemini/erd_skema_database_postgresql.md:162-170
 */
class StoreTargetRequest extends FormRequest
{
    public function authorize(): bool
    {
        return true;
    }

    public function rules(): array
    {
        return [
            'name' => ['required', 'string', 'max:150'],
            'dashboard_group' => ['required', Rule::in(['NETWORK', 'APP_DOMAIN_DB', 'MAIL_S3_BESZEL'])],
            'target_type' => ['required', Rule::in([
                'NETWORK_DEVICE', 'WEB_APP', 'DOMAIN', 'DATABASE', 'MAIL_SERVER', 'S3_STORAGE', 'BESZEL_SERVER',
            ])],
            'environment' => ['required', Rule::in(['PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL'])],
            'host_or_endpoint' => ['required', 'string', 'max:255'],
            'port' => ['nullable', 'integer', 'between:1,65535'],
            'protocol_type' => ['required', Rule::in([
                'ICMP_SNMP', 'HTTP_HEALTH', 'WHOIS_SSL', 'DB_NATIVE_PING', 'SMTP_DNSBL', 'S3_HEAD', 'BESZEL_API',
            ])],
            'config_payload' => ['array'],
            'check_interval_seconds' => ['integer', 'min:10'],
            'timeout_seconds' => ['integer', 'between:1,60'],
            'retry_threshold' => ['integer', 'between:1,10'],
            'degraded_threshold_ms' => ['integer', 'min:1'],
            'credentials' => ['nullable', 'array'],
            'credentials.auth_type' => ['nullable', Rule::in([
                'NONE', 'DB_AUTH', 'SNMP_V2C', 'SNMP_V3', 'S3_KEY', 'BESZEL_TOKEN', 'HTTP_BEARER',
            ])],
        ];
    }
}
