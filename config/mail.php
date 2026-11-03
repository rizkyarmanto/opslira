<?php

return [
    'default' => env('MAIL_MAILER', 'smtp'),
    'mailers' => [
        'smtp' => [
            'transport' => 'smtp',
            'scheme' => env('MAIL_SCHEME'),
            'url' => env('MAIL_URL'),
            'host' => env('MAIL_HOST', '127.0.0.1'),
            'port' => env('MAIL_PORT', 2525),
            'username' => env('MAIL_USERNAME'),
            'password' => env('MAIL_PASSWORD'),
            'timeout' => env('MAIL_TIMEOUT', 10),
            // Symfony disables certificate and peer-name checks only when this is
            // explicitly false; the default keeps TLS verification on for any
            // relay with a real certificate.
            'verify_peer' => env('MAIL_VERIFY_PEER', true),
            'local_domain' => env('MAIL_EHLO_DOMAIN', parse_url(env('APP_URL', 'http://localhost'), PHP_URL_HOST)),
        ],
        'log' => [
            'transport' => 'log',
            'channel' => env('MAIL_LOG_CHANNEL'),
        ],
        'array' => [
            'transport' => 'array',
        ],
        'failover' => [
            'transport' => 'failover',
            'mailers' => ['smtp', 'log'],
        ],
    ],
    'from' => [
        'address' => env('MAIL_FROM_ADDRESS', 'opslira@kantor.co.id'),
        'name' => env('MAIL_FROM_NAME', 'Opslira Monitoring'),
    ],
];
