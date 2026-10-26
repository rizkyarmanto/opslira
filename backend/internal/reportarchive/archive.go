// Package reportarchive uploads generated report artifacts (PDF/XLSX) to an
// S3-compatible bucket (RustFS). It is best-effort: when the endpoint is
// unconfigured the upload is skipped, and a failed upload never fails the
// report run — the local file stays the source of truth.
package reportarchive

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"opslira/backend/internal/config"
)

// Archiver uploads report files under <prefix>/reports/YYYY-MM/<name>.
type Archiver struct {
	cfg    config.ReportS3
	client *s3.Client
}

// New builds the uploader, or nil when REPORT_S3_ENDPOINT is empty (upload
// disabled). Missing bucket/keys surface at upload time, not at boot, so a
// half-configured env never blocks the API from starting.
func New(cfg config.ReportS3) *Archiver {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	return &Archiver{cfg: cfg}
}

func (a *Archiver) clientFor(ctx context.Context) (*s3.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	region := a.cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(a.cfg.AccessKey, a.cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}
	a.client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(a.cfg.Endpoint)
		// S3-compatible stores (RustFS/MinIO) lack wildcard DNS for
		// virtual-host addressing.
		o.UsePathStyle = true
	})
	return a.client, nil
}

// Put uploads content to <prefix>/<key> and returns the object key. Callers
// pass the local relative path (reports/YYYY-MM/<PERIOD>-....<ext>) so the
// bucket layout mirrors REPORT_STORAGE_DIR.
func (a *Archiver) Put(ctx context.Context, key string, content []byte, contentType string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("report archive: uploader disabled")
	}
	if strings.TrimSpace(a.cfg.Bucket) == "" {
		return "", fmt.Errorf("report archive: REPORT_S3_BUCKET is empty")
	}
	if strings.TrimSpace(a.cfg.AccessKey) == "" || strings.TrimSpace(a.cfg.SecretKey) == "" {
		return "", fmt.Errorf("report archive: REPORT_S3_ACCESS_KEY/SECRET_KEY is empty")
	}
	client, err := a.clientFor(ctx)
	if err != nil {
		return "", err
	}
	objectKey := strings.Trim(a.cfg.Prefix+"/"+strings.Trim(key, "/"), "/")

	putCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, err := client.PutObject(putCtx, &s3.PutObjectInput{
		Bucket:      aws.String(a.cfg.Bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("report archive upload %s: %w", objectKey, err)
	}
	return objectKey, nil
}
