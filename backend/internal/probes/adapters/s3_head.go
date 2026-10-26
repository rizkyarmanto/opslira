package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"opslira/backend/internal/probes"
)

// S3Head probes S3_STORAGE targets: HeadBucket + ListObjectsV2 (max 1 key),
// against AWS or a compatible endpoint (S3BucketProbeAdapter).
type S3Head struct{ adapterBase }

// NewS3Head builds the S3_HEAD adapter.
func NewS3Head() *S3Head { return &S3Head{adapterBase: newBase()} }

func (a *S3Head) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolS3Head
}

func (a *S3Head) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *S3Head) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

func (a *S3Head) Execute(ctx context.Context) probes.ProbeResult {
	bucket := a.cfgString("bucket_name", "")
	if bucket == "" {
		return probes.Down("Missing bucket_name in config_payload", nil, nil)
	}
	region := a.cfgString("region", "us-east-1")

	key := a.credString("access_key")
	secret := a.credString("secret_key")
	if key == "" || secret == "" {
		return probes.Down("Missing S3 credentials", nil, nil)
	}

	provider := a.cfgString("s3_provider", "aws")
	legacyEndpoint := strings.TrimSpace(a.cfgString("endpoint", ""))
	host := strings.TrimSuffix(strings.TrimSpace(a.cfgString("host", "")), "/")

	var endpoint string
	switch {
	case legacyEndpoint != "":
		// Rows written by the old form keep working; the form no longer
		// writes this key (the S3 URL field is the endpoint now).
		endpoint = legacyEndpoint
	case provider == "compatible":
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			return probes.Down(
				"S3 compatible provider needs an S3 URL host starting with http:// or https://",
				map[string]any{"bucket_accessible": false},
				nil,
			)
		}
		endpoint = host
	}

	// Custom endpoints lack wildcard DNS for virtual-host addressing.
	pathStyle := provider == "compatible" || a.cfgBool("use_path_style_endpoint", false)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")),
	)
	if err != nil {
		return probes.Down("S3 probe error: "+err.Error(), map[string]any{"bucket_accessible": false}, nil)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		options.UsePathStyle = pathStyle
	})

	start := time.Now()
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	headMs := round2(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		return probes.Down("S3 error: "+awsErrorMessage(err), map[string]any{"bucket_accessible": false}, nil)
	}

	metrics := map[string]any{
		"bucket_accessible": true,
		"head_bucket_ms":    headMs,
		"signature_version": "v4",
	}

	listStart := time.Now()
	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket, MaxKeys: aws.Int32(1)})
	if err != nil {
		// The key can reach the bucket (HeadBucket) but lack s3:ListBucket.
		metrics["bucket_listable"] = false
		return probes.Down("S3 list error: "+awsErrorMessage(err), metrics, msPointer(headMs))
	}
	metrics["list_objects_ms"] = round2(float64(time.Since(listStart).Microseconds()) / 1000)
	metrics["bucket_listable"] = true

	status := a.classifyLatency(msPointer(headMs))
	return probes.ProbeResult{Status: status, ResponseTimeMs: msPointer(headMs), MetricsPayload: metrics}
}

// awsErrorMessage is the AWS SDK message (getAwsErrorMessage), falling back
// to the wrapped error text.
func awsErrorMessage(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorMessage() != "" {
		return apiErr.ErrorMessage()
	}
	return err.Error()
}
