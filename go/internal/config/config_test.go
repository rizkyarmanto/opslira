package config

import "testing"

func TestLoadRejectsMissingToken(t *testing.T) {
	t.Setenv("OPSLIRA_RUNNER_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() with no OPSLIRA_RUNNER_TOKEN succeeded, want an error")
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "non-integer concurrency", key: "PROBE_CONCURRENCY", value: "eight"},
		{name: "zero tick", key: "RUNNER_TICK_SECONDS", value: "0"},
		{name: "malformed rollup time", key: "ROLLUP_AT", value: "8am"},
		{name: "out of range prune time", key: "PRUNE_AT", value: "25:00"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("OPSLIRA_RUNNER_TOKEN", "test-token")
			t.Setenv(test.key, test.value)

			if _, err := Load(); err == nil {
				t.Errorf("Load() with %s=%q succeeded, want an error", test.key, test.value)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPSLIRA_RUNNER_TOKEN", "test-token")
	for _, key := range []string{
		"OPSLIRA_BASE_URL", "PROBE_CONCURRENCY", "RUNNER_TICK_SECONDS",
		"REPORT_TICK_SECONDS", "ROLLUP_AT", "PRUNE_AT", "RUNNER_LISTEN_ADDR", "RUNNER_TIMEZONE",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.BaseURL != "http://127.0.0.1:8080" {
		t.Errorf("BaseURL = %q, want the default", cfg.BaseURL)
	}
	if cfg.ProbeConcurrency != 8 || cfg.TickSeconds != 15 || cfg.ReportTickSeconds != 60 {
		t.Errorf("concurrency/ticks = %d/%d/%d, want 8/15/60",
			cfg.ProbeConcurrency, cfg.TickSeconds, cfg.ReportTickSeconds)
	}
	if cfg.RollupAt != "00:05" || cfg.PruneAt != "02:00" || cfg.ListenAddr != ":9090" || cfg.Timezone != "Asia/Jakarta" {
		t.Errorf("daily/listen/timezone defaults = %q/%q/%q/%q",
			cfg.RollupAt, cfg.PruneAt, cfg.ListenAddr, cfg.Timezone)
	}
}
