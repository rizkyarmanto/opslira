<?php

use App\Modules\Targets\Services\DatabaseActivityService;

it('orders database activity by the greatest raw duration while preserving ties and nulls', function () {
    $row = static fn ($name, $state, $query) => [
        'username' => $name,
        'database' => null,
        'state' => 'idle',
        'state_seconds' => $state,
        'query_seconds' => $query,
        'query_syntax' => null,
    ];
    $rows = [
        $row('query', '10', '100'),
        $row('state', '90', null),
        $row('oldest', null, '120'),
        $row('unknown', null, null),
        $row('fraction-first', '100.5', null),
        $row('fraction-second', null, '100.5'),
    ];

    $format = new ReflectionMethod(DatabaseActivityService::class, 'formatRows');
    $result = $format->invoke(null, $rows);

    expect(array_column($result, 'username'))->toBe([
        'oldest', 'fraction-first', 'fraction-second', 'query', 'state', 'unknown',
    ])->and($result[0])->toBe([
        'username' => 'oldest',
        'database' => null,
        'state' => 'idle',
        'state_duration' => null,
        'query_age' => '0d 00h 02m 00s',
        'query_syntax' => null,
    ])->and($result[3]['state_duration'])->toBe('0d 00h 00m 10s')
        ->and($result[3]['query_age'])->toBe('0d 00h 01m 40s')
        ->and($result[5]['state_duration'])->toBeNull()
        ->and($result[5]['query_age'])->toBeNull();
});
