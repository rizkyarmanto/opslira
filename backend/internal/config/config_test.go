package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.ProbeConcurrency != 8 {
		t.Errorf("ProbeConcurrency = %d, want 8", cfg.ProbeConcurrency)
	}
	if cfg.TickSeconds != 15 {
		t.Errorf("TickSeconds = %d, want 15", cfg.TickSeconds)
	}
	if cfg.ReportTickSeconds != 60 {
		t.Errorf("ReportTickSeconds = %d, want 60", cfg.ReportTickSeconds)
	}
	if cfg.RollupAt != "00:05" || cfg.PruneAt != "02:00" {
		t.Errorf("ROLLUP_AT/PRUNE_AT = %q/%q", cfg.RollupAt, cfg.PruneAt)
	}
	if cfg.Timezone != "Asia/Jakarta" {
		t.Errorf("Timezone = %q", cfg.Timezone)
	}
	if cfg.SessionTTL != 86400*time.Second {
		t.Errorf("SessionTTL = %s, want 24h", cfg.SessionTTL)
	}
	if cfg.BeszelTokenCacheTTL != 43200*time.Second {
		t.Errorf("BeszelTokenCacheTTL = %s, want 43200s", cfg.BeszelTokenCacheTTL)
	}
	if cfg.Redis.DB != 0 || cfg.Redis.CacheDB != 1 {
		t.Errorf("Redis DBs = %d/%d, want 0/1", cfg.Redis.DB, cfg.Redis.CacheDB)
	}
	if cfg.ReportS3.Endpoint != "" || cfg.ReportS3.Bucket != "" {
		t.Errorf("ReportS3 defaults = %+v, want empty endpoint/bucket", cfg.ReportS3)
	}
	if cfg.ReportS3.Region != "us-east-1" {
		t.Errorf("ReportS3.Region = %q, want us-east-1", cfg.ReportS3.Region)
	}
	if loc := cfg.Location(); loc.String() != "Asia/Jakarta" {
		t.Errorf("Location() = %q", loc.String())
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	cases := map[string]string{
		"PROBE_CONCURRENCY": "not-a-number",
		"ROLLUP_AT":         "25:00",
		"PRUNE_AT":          "00:61",
		"RUNNER_TIMEZONE":   "Mars/Olympus",
	}
	for key, value := range cases {
		t.Run(key+"="+value, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Errorf("Load accepted %s=%q", key, value)
			}
		})
	}
}

func TestLoadRejectsConcurrencyBelowOne(t *testing.T) {
	t.Setenv("PROBE_CONCURRENCY", "0")
	if _, err := Load(); err == nil {
		t.Error("Load accepted PROBE_CONCURRENCY=0")
	}
}
