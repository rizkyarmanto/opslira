<?php

namespace App\Modules\Targets\Requests;

use Illuminate\Contracts\Validation\Validator;
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
                'NETWORK_DEVICE', 'WEB_APP', 'DOMAIN', 'DATABASE', 'MAIL_SERVER', 'S3_STORAGE', 'BESZEL_SERVER', 'INTERNET',
            ])],
            'environment' => ['required', Rule::in(['PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL'])],
            'host_or_endpoint' => ['required', 'string', 'max:255'],
            'port' => ['nullable', 'integer', 'between:1,65535'],
            'protocol_type' => ['required', Rule::in([
                'ICMP_SNMP', 'HTTP_HEALTH', 'WHOIS_SSL', 'DB_NATIVE_PING', 'SMTP_DNSBL', 'S3_HEAD', 'BESZEL_API', 'ICMP_PING',
            ])],
            'config_payload' => ['array'],
            'check_interval_seconds' => ['integer', 'min:10'],
            'timeout_seconds' => ['integer', 'between:1,60'],
            'retry_threshold' => ['integer', 'between:1,10'],
            'degraded_threshold_ms' => ['integer', 'min:1'],
            'credentials' => ['nullable', 'array'],
            'credentials.auth_type' => ['nullable', Rule::in([
                'NONE', 'DB_AUTH', 'SNMP_V2C', 'SNMP_V3', 'S3_KEY', 'BESZEL', 'HTTP_BASIC', 'HTTP_BEARER', 'MAIL_AUTH',
            ])],
            'credentials.payload' => ['nullable', 'array'],
            'credentials.payload.community' => [Rule::requiredIf($this->authType() === 'SNMP_V2C'), 'string', 'max:255'],
            'credentials.payload.username' => [
                Rule::requiredIf(in_array($this->authType(), ['HTTP_BASIC', 'DB_AUTH', 'SNMP_V3'], true)),
                'string', 'max:255',
            ],
            'credentials.payload.password' => [
                Rule::requiredIf(in_array($this->authType(), ['HTTP_BASIC', 'DB_AUTH'], true)),
                'string', 'max:255',
            ],
            'credentials.payload.auth_protocol' => [
                Rule::requiredIf($this->authType() === 'SNMP_V3'), Rule::in(['MD5', 'SHA']),
            ],
            'credentials.payload.auth_password' => [Rule::requiredIf($this->authType() === 'SNMP_V3'), 'string'],
            'credentials.payload.priv_protocol' => [
                Rule::requiredIf($this->authType() === 'SNMP_V3'), Rule::in(['DES', 'AES']),
            ],
            'credentials.payload.priv_password' => [Rule::requiredIf($this->authType() === 'SNMP_V3'), 'string'],
            'credentials.payload.bearer_token' => [Rule::requiredIf($this->authType() === 'HTTP_BEARER'), 'string', 'max:2048'],
            'credentials.payload.access_key' => [Rule::requiredIf($this->authType() === 'S3_KEY'), 'string', 'max:255'],
            'credentials.payload.secret_key' => [Rule::requiredIf($this->authType() === 'S3_KEY'), 'string', 'max:512'],
            'credentials.payload.smtp_username' => ['nullable', 'string', 'max:255'],
            'credentials.payload.smtp_password' => ['nullable', 'string', 'max:255'],
            'credentials.payload.imap_username' => ['nullable', 'string', 'max:255'],
            'credentials.payload.imap_password' => ['nullable', 'string', 'max:255'],
        ];
    }

    /**
     * Beszel credentials are XOR: an API token, or both email and password.
     */
    public function withValidator(Validator $validator): void
    {
        $validator->after(function (Validator $validator) {
            $authType = $this->authType();
            $payload = (array) $this->input('credentials.payload', []);

            if ($authType === 'BESZEL') {
                $hasToken = ! empty($payload['api_token']);
                $hasLogin = ! empty($payload['email']) && ! empty($payload['password']);

                if (! $hasToken && ! $hasLogin || $hasToken && $hasLogin) {
                    $validator->errors()->add(
                        'credentials.payload',
                        'Beszel credentials require either an API token or both email and password.'
                    );
                }
            }

            if ($authType === 'MAIL_AUTH') {
                // One row per target, so the login choice decides which pair is
                // present; both pairs together stay valid for raw API callers.
                $hasSmtp = ! empty($payload['smtp_username']) && ! empty($payload['smtp_password']);
                $hasImap = ! empty($payload['imap_username']) && ! empty($payload['imap_password']);

                if (! $hasSmtp && ! $hasImap) {
                    $validator->errors()->add(
                        'credentials.payload',
                        'SMTP/IMAP login requires both an SMTP username and password, or both an IMAP username and password.'
                    );
                }
            }
        });
    }

    private function authType(): ?string
    {
        return $this->input('credentials.auth_type');
    }
}
