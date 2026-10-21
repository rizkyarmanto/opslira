<?php

namespace Database\Seeders;

use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Database\Seeder;
use Illuminate\Support\Str;

class MonitoringTargetSeeder extends Seeder
{
    public function run(): void
    {
        $targets = [
            [
                'name' => 'Office Core Router',
                'dashboard_group' => 'NETWORK',
                'target_type' => 'NETWORK_DEVICE',
                'environment' => 'INTERNAL',
                'host_or_endpoint' => '192.168.1.1',
                'protocol_type' => 'ICMP_SNMP',
                'config_payload' => [
                    'sub_type' => 'ROUTER',
                    'icmp_count' => 4,
                    'enable_snmp' => true,
                    'snmp_version' => '2c',
                    'interfaces' => ['ether1-wan', 'bridge-lan'],
                ],
            ],
            [
                'name' => 'ERP Portal Production',
                'dashboard_group' => 'APP_DOMAIN_DB',
                'target_type' => 'WEB_APP',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => 'https://erp.kantor.co.id/health',
                'protocol_type' => 'HTTP_HEALTH',
                'config_payload' => [
                    'method' => 'GET',
                    'expected_status' => 200,
                    'verify_ssl' => true,
                    'expected_json_key' => 'status',
                    'expected_json_value' => 'ok',
                ],
            ],
            [
                'name' => 'kantor.co.id Domain',
                'dashboard_group' => 'APP_DOMAIN_DB',
                'target_type' => 'DOMAIN',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => 'kantor.co.id',
                'protocol_type' => 'WHOIS_SSL',
                'config_payload' => [
                    'whois_server' => 'whois.id',
                    'check_ssl_port' => 443,
                    'expected_a_records' => [],
                    'warn_days_before' => 14,
                ],
            ],
            [
                'name' => 'ERP Database Production',
                'dashboard_group' => 'APP_DOMAIN_DB',
                'target_type' => 'DATABASE',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => '10.10.0.20',
                'port' => 5432,
                'protocol_type' => 'DB_NATIVE_PING',
                'config_payload' => [
                    'db_driver' => 'pgsql',
                    'database_name' => 'erp_prod',
                    'query' => 'SELECT 1',
                    'collect_connection_stats' => true,
                ],
            ],
            [
                'name' => 'Office Mail Server',
                'dashboard_group' => 'MAIL_S3_BESZEL',
                'target_type' => 'MAIL_SERVER',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => 'mail.kantor.co.id',
                'protocol_type' => 'SMTP_DNSBL',
                'config_payload' => [
                    'check_ports' => ['smtp' => 587, 'imap' => 993],
                    'check_dnsbl' => true,
                    'dnsbl_providers' => ['zen.spamhaus.org', 'bl.spamcop.net'],
                ],
            ],
            [
                'name' => 'Backup Bucket Production',
                'dashboard_group' => 'MAIL_S3_BESZEL',
                'target_type' => 'S3_STORAGE',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => 'https://s3.ap-southeast-1.amazonaws.com',
                'protocol_type' => 'S3_HEAD',
                'config_payload' => [
                    'region' => 'ap-southeast-1',
                    'bucket_name' => 'kantor-backup-prod',
                    'use_path_style_endpoint' => true,
                ],
            ],
            [
                'name' => 'Beszel Hub Node-01',
                'dashboard_group' => 'MAIL_S3_BESZEL',
                'target_type' => 'BESZEL_SERVER',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => 'http://beszel.kantor.co.id:8090',
                'protocol_type' => 'BESZEL_API',
                'config_payload' => [
                    'beszel_system_id' => 'rec_xyz123456',
                    'warn_cpu_pct' => 85,
                    'warn_mem_pct' => 90,
                    'warn_disk_pct' => 85,
                ],
            ],
            [
                'name' => 'Internet DNS (8.8.8.8)',
                'dashboard_group' => 'NETWORK',
                'target_type' => 'INTERNET',
                'environment' => 'PRODUCTION',
                'host_or_endpoint' => '8.8.8.8',
                'protocol_type' => 'ICMP_PING',
                // icmp_only is written explicitly because the seeder bypasses the
                // controller, and without it the target inherits the TCP fallback.
                'config_payload' => [
                    'icmp_count' => 4,
                    'icmp_only' => true,
                ],
            ],
        ];

        foreach ($targets as $target) {
            MonitoringTarget::updateOrCreate(
                ['name' => $target['name']],
                [...$target, 'check_interval_seconds' => 60],
            );
        }
    }
}
