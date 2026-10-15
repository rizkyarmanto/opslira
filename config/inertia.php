<?php

return [
    'ssr' => [
        'enabled' => (bool) env('INERTIA_SSR_ENABLED', false),
        'url' => env('INERTIA_SSR_URL', 'http://127.0.0.1:13714'),
    ],
    'testing' => [
        'ensure_pages_exist' => false,
        'page_paths' => [
            resource_path('js/pages'),
        ],
        'page_extensions' => [
            'js', 'jsx', 'svelte', 'ts', 'tsx', 'vue',
        ],
    ],
];
