// Package config loads the runner's settings from the environment.
//
// Every value is either a documented default or a parse error: a malformed
// integer or clock time stops the process instead of silently falling back to a
// default the operator did not ask for.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Zoneinfo is embedded so the binary resolves RUNNER_TIMEZONE even on a
	// minimal image without a system tzdata package.
	_ "time/tzdata"
)

type Config struct {
	BaseURL           string
	Token             string
	ProbeConcurrency  int
	TickSeconds       int
	ReportTickSeconds int
	RollupAt          string
	PruneAt           string
	ListenAddr        string
	Timezone          string
}

func Load() (Config, error) {
	cfg := Config{
		BaseURL:     envOr("OPSLIRA_BASE_URL", "http://127.0.0.1:8080"),
		Token:       strings.TrimSpace(os.Getenv("OPSLIRA_RUNNER_TOKEN")),
		RollupAt:    envOr("ROLLUP_AT", "00:05"),
		PruneAt:     envOr("PRUNE_AT", "02:00"),
		ListenAddr:  envOr("RUNNER_LISTEN_ADDR", ":9090"),
		Timezone:    envOr("RUNNER_TIMEZONE", "Asia/Jakarta"),
	}

	if cfg.Token == "" {
		return Config{}, fmt.Errorf("OPSLIRA_RUNNER_TOKEN is required")
	}

	var err error
	if cfg.ProbeConcurrency, err = intEnv("PROBE_CONCURRENCY", 8); err != nil {
		return Config{}, err
	}
	if cfg.TickSeconds, err = intEnv("RUNNER_TICK_SECONDS", 15); err != nil {
		return Config{}, err
	}
	if cfg.ReportTickSeconds, err = intEnv("REPORT_TICK_SECONDS", 60); err != nil {
		return Config{}, err
	}
	if cfg.ProbeConcurrency < 1 {
		return Config{}, fmt.Errorf("PROBE_CONCURRENCY must be at least 1, got %d", cfg.ProbeConcurrency)
	}
	if cfg.TickSeconds < 1 || cfg.ReportTickSeconds < 1 {
		return Config{}, fmt.Errorf("tick intervals must be at least 1 second")
	}

	if err := validateHHMM(cfg.RollupAt); err != nil {
		return Config{}, fmt.Errorf("ROLLUP_AT: %w", err)
	}
	if err := validateHHMM(cfg.PruneAt); err != nil {
		return Config{}, fmt.Errorf("PRUNE_AT: %w", err)
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return Config{}, fmt.Errorf("RUNNER_TIMEZONE: %w", err)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}

	return value, nil
}

func validateHHMM(value string) error {
	if len(value) != 5 || value[2] != ':' {
		return fmt.Errorf("must be HH:MM, got %q", value)
	}

	hour, err := strconv.Atoi(value[0:2])
	if err != nil {
		return fmt.Errorf("must be HH:MM, got %q", value)
	}
	minute, err := strconv.Atoi(value[3:5])
	if err != nil {
		return fmt.Errorf("must be HH:MM, got %q", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("must be a real clock time in HH:MM, got %q", value)
	}

	return nil
}
