<?php

return [
    'postmark' => [
        'token' => env('POSTMARK_TOKEN'),
    ],
    'ses' => [
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'region' => env('AWS_DEFAULT_REGION', 'us-east-1'),
    ],
    'gotenberg' => [
        'url' => env('GOTENBERG_URL', 'http://monitoring-gotenberg:3000'),
    ],
    'beszel' => [
        'hub_url' => env('BESZEL_HUB_URL'),
        'token_cache_ttl' => env('BESZEL_TOKEN_CACHE_TTL', 43200), // 12 hours
    ],
    'telegram' => [
        'bot_token' => env('TELEGRAM_BOT_TOKEN'),
        'chat_id' => env('TELEGRAM_CHAT_ID'),
    ],
    'slack' => [
        'webhook_url' => env('SLACK_WEBHOOK_URL'),
    ],
];
