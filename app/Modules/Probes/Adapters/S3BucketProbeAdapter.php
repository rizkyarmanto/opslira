<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Aws\S3\S3Client;
use Aws\Exception\AwsException;

/**
 * S3_STORAGE: AWS S3 SDK v3 HeadBucket + ListObjectsV2 (max 1).
 */
class S3BucketProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'S3_HEAD';
    }

    public function execute(): ProbeResult
    {
        $bucket = $this->config('bucket_name');
        $region = (string) $this->config('region', 'us-east-1');

        if (! $bucket) {
            return ProbeResult::down('Missing bucket_name in config_payload');
        }

        $key = (string) $this->credential('access_key', '');
        $secret = (string) $this->credential('secret_key', '');

        if ($key === '' || $secret === '') {
            return ProbeResult::down('Missing S3 credentials');
        }

        $options = [
            'version' => 'latest',
            'region' => $region,
            'credentials' => ['key' => $key, 'secret' => $secret],
        ];

        $provider = (string) ($this->config('s3_provider') ?: 'aws');
        $legacyEndpoint = trim((string) $this->config('endpoint', ''));
        $host = rtrim(trim((string) $this->config('host', '')), '/');

        if ($legacyEndpoint !== '') {
            // Rows written by the old form keep working; the form no longer
            // writes this key (the S3 URL field is the endpoint now).
            $endpoint = $legacyEndpoint;
        } elseif ($provider === 'compatible') {
            if (! str_starts_with($host, 'http://') && ! str_starts_with($host, 'https://')) {
                return ProbeResult::down('S3 compatible provider needs an S3 URL host starting with http:// or https://', ['bucket_accessible' => false]);
            }
            $endpoint = $host;
        } else {
            $endpoint = '';
        }
        if ($endpoint !== '') {
            $options['endpoint'] = $endpoint;
        }

        // Custom endpoints lack wildcard DNS for virtual-host addressing.
        if ($provider === 'compatible' ? true : (bool) $this->config('use_path_style_endpoint', false)) {
            $options['use_path_style_endpoint'] = true;
        }

        try {
            $client = new S3Client($options);

            $start = microtime(true);
            $client->headBucket(['Bucket' => $bucket]);
            $headMs = round((microtime(true) - $start) * 1000, 2);

            $metrics = [
                'bucket_accessible' => true,
                'head_bucket_ms' => $headMs,
                'signature_version' => 'v4',
            ];

            try {
                $listStart = microtime(true);
                $client->listObjectsV2(['Bucket' => $bucket, 'MaxKeys' => 1]);
                $metrics['list_objects_ms'] = round((microtime(true) - $listStart) * 1000, 2);
                $metrics['bucket_listable'] = true;
            } catch (AwsException $e) {
                // The key can reach the bucket (HeadBucket) but lack s3:ListBucket.
                $awsMessage = $e->getAwsErrorMessage() ?: $e->getMessage();

                return ProbeResult::down(
                    'S3 list error: '.$awsMessage,
                    [...$metrics, 'bucket_listable' => false],
                    $headMs,
                );
            }

            return new ProbeResult($this->classifyLatency($headMs), $headMs, null, $metrics);
        } catch (AwsException $e) {
            $awsMessage = $e->getAwsErrorMessage() ?: $e->getMessage();

            return ProbeResult::down('S3 error: '.$awsMessage, ['bucket_accessible' => false]);
        } catch (\Throwable $e) {
            return ProbeResult::down('S3 probe error: '.$e->getMessage(), ['bucket_accessible' => false]);
        }
    }
}
