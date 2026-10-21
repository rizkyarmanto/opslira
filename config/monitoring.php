<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Credential Encryption Key
    |--------------------------------------------------------------------------
    |
    | AES-256-GCM key for target credentials. Falls back to APP_KEY when the
    | dedicated key is not set.
    |
    */

    'credential_encryption_key' => env('APP_ENCRYPTION_KEY', ''),

    /*
    |--------------------------------------------------------------------------
    | Alert Recipients
    |--------------------------------------------------------------------------
    |
    | Comma-separated list of e-mail addresses that receive incident alerts.
    |
    */

    'alert_recipients' => array_filter(array_map('trim', explode(',', (string) env('ALERT_RECIPIENTS', '')))),

    /*
    |--------------------------------------------------------------------------
    | Runner Token
    |--------------------------------------------------------------------------
    |
    | Shared secret the Go runner sends in X-Opslira-Runner. When empty, the
    | internal endpoints reject every request.
    |
    */

    'runner_token' => env('OPSLIRA_RUNNER_TOKEN', ''),

];
