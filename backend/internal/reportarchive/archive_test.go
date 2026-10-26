package reportarchive

import (
	"testing"

	"opslira/backend/internal/config"
)

func TestNewDisabledWithoutEndpoint(t *testing.T) {
	if New(config.ReportS3{Bucket: "reports"}) != nil {
		t.Error("New with empty endpoint should return nil")
	}
}

func TestPutDisabledUploader(t *testing.T) {
	var a *Archiver
	if _, err := a.Put(t.Context(), "reports/x.pdf", []byte("x"), "application/pdf"); err == nil {
		t.Error("Put on nil archiver should error")
	}
}

func TestPutRequiresBucketAndKeys(t *testing.T) {
	a := New(config.ReportS3{Endpoint: "https://s3-api.vezaro.biz.id"})
	if a == nil {
		t.Fatal("New should return archiver when endpoint set")
	}
	if _, err := a.Put(t.Context(), "reports/x.pdf", []byte("x"), "application/pdf"); err == nil {
		t.Error("Put without bucket/keys should error")
	}
}
