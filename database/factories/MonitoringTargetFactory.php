<?php

namespace Database\Factories;

use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Database\Eloquent\Factories\Factory;

/**
 * @extends Factory<MonitoringTarget>
 */
class MonitoringTargetFactory extends Factory
{
    protected $model = MonitoringTarget::class;

    public function definition(): array
    {
        return [
            'name' => fake()->unique()->domainWord().' target',
            'dashboard_group' => 'NETWORK',
            'target_type' => 'NETWORK_DEVICE',
            'environment' => 'INTERNAL',
            'host_or_endpoint' => fake()->ipv4(),
            'protocol_type' => 'ICMP_SNMP',
            'config_payload' => ['sub_type' => 'ROUTER', 'icmp_count' => 4, 'enable_snmp' => false],
            'check_interval_seconds' => 60,
            'timeout_seconds' => 5,
            'retry_threshold' => 3,
            'degraded_threshold_ms' => 1000,
            'current_status' => 'PENDING',
            'is_maintenance' => false,
            'is_active' => true,
            'next_check_at' => now(),
        ];
    }

    public function webApp(): static
    {
        return $this->state(fn () => [
            'dashboard_group' => 'APP_DOMAIN_DB',
            'target_type' => 'WEB_APP',
            'environment' => 'PRODUCTION',
            'host_or_endpoint' => 'https://'.fake()->domainName().'/health',
            'protocol_type' => 'HTTP_HEALTH',
            'config_payload' => ['method' => 'GET', 'expected_status' => 200, 'verify_ssl' => true],
        ]);
    }
}
