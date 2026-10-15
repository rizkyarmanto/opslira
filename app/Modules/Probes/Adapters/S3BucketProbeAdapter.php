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

        if ((bool) $this->config('use_path_style_endpoint', false)) {
            $options['use_path_style_endpoint'] = true;
        }

        $endpoint = $this->config('endpoint');
        if ($endpoint) {
            $options['endpoint'] = $endpoint;
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

            return new ProbeResult($this->classifyLatency($headMs), $headMs, null, $metrics);
        } catch (AwsException $e) {
            return ProbeResult::down('S3 error: '.$e->getAwsErrorMessage(), ['bucket_accessible' => false]);
        } catch (\Throwable $e) {
            return ProbeResult::down('S3 probe error: '.$e->getMessage(), ['bucket_accessible' => false]);
        }
    }
}
