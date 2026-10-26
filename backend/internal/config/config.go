// Package config loads the API's settings from the environment.
//
// Every value is either a documented default or a parse error: a malformed
// integer or clock time stops the process instead of silently falling back to
// a default the operator did not ask for.
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

// DB is the Postgres 16 connection configuration.
type DB struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

// Redis is the Redis 7 connection configuration. DB holds data keys
// (sessions, probe state); CacheDB holds short-lived dashboard caches.
type Redis struct {
	Host     string
	Port     string
	Password string
	DB       int
	CacheDB  int
}

// ReportS3 is the optional archive sink for generated reports: when Endpoint
// is empty the generator skips the upload and keeps local files only.
type ReportS3 struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
	Prefix    string
}

// Mail is the outgoing SMTP configuration.
type Mail struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	FromName    string
	// Scheme: "smtps" for implicit TLS (port 465); empty means STARTTLS
	// when the server advertises it.
	Scheme string
}

// Config is the fully resolved runtime configuration.
type Config struct {
	ListenAddr       string
	FrontendOrigin   string
	SessionTTL       time.Duration
	DB               DB
	Redis            Redis
	GotenbergURL     string
	ReportS3         ReportS3
	AppKey           string
	AppEncryptionKey string

	ProbeConcurrency  int
	TickSeconds       int
	ReportTickSeconds int
	RollupAt          string
	PruneAt           string
	Timezone          string

	BeszelTokenCacheTTL time.Duration

	AlertRecipients  []string
	Mail             Mail
	TelegramBotToken string
	TelegramChatID   string
	SlackWebhookURL  string

	AdminEmail    string
	AdminName     string
	AdminPassword string
}

// Load resolves configuration from environment variables. Parse errors in
// integers, clock times or the timezone abort startup.
func Load() (Config, error) {
	cfg := Config{
		ListenAddr:     envOr("API_LISTEN_ADDR", ":8080"),
		FrontendOrigin: envOr("FRONTEND_ORIGIN", "http://localhost"),
		GotenbergURL:   strings.TrimRight(envOr("GOTENBERG_URL", "http://127.0.0.1:3000"), "/"),
		ReportS3: ReportS3{
			Endpoint:  strings.TrimRight(strings.TrimSpace(os.Getenv("REPORT_S3_ENDPOINT")), "/"),
			Bucket:    strings.TrimSpace(os.Getenv("REPORT_S3_BUCKET")),
			AccessKey: os.Getenv("REPORT_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("REPORT_S3_SECRET_KEY"),
			Region:    envOr("REPORT_S3_REGION", "us-east-1"),
			Prefix:    strings.Trim(strings.TrimSpace(os.Getenv("REPORT_S3_PREFIX")), "/"),
		},
		AppKey:           strings.TrimSpace(os.Getenv("APP_KEY")),
		AppEncryptionKey: strings.TrimSpace(os.Getenv("APP_ENCRYPTION_KEY")),
		RollupAt:         envOr("ROLLUP_AT", "00:05"),
		PruneAt:          envOr("PRUNE_AT", "02:00"),
		Timezone:         envOr("RUNNER_TIMEZONE", "Asia/Jakarta"),
		DB: DB{
			Host:     envOr("DB_HOST", "127.0.0.1"),
			Port:     envOr("DB_PORT", "5432"),
			Name:     envOr("DB_DATABASE", "internal_monitoring"),
			User:     envOr("DB_USERNAME", "mon_admin"),
			Password: os.Getenv("DB_PASSWORD"),
			SSLMode:  envOr("DB_SSLMODE", "disable"),
		},
		Redis: Redis{
			Host:     envOr("REDIS_HOST", "127.0.0.1"),
			Port:     envOr("REDIS_PORT", "6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
		},
		Mail: Mail{
			Host:        envOr("MAIL_HOST", ""),
			Port:        envOr("MAIL_PORT", "587"),
			Username:    os.Getenv("MAIL_USERNAME"),
			Password:    os.Getenv("MAIL_PASSWORD"),
			FromAddress: envOr("MAIL_FROM_ADDRESS", "opslira@kantor.co.id"),
			FromName:    envOr("MAIL_FROM_NAME", "Opslira Monitoring"),
			Scheme:      envOr("MAIL_SCHEME", ""),
		},
		AlertRecipients:  splitCSV(os.Getenv("ALERT_RECIPIENTS")),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		SlackWebhookURL:  strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL")),
		AdminEmail:       envOr("ADMIN_EMAIL", "admin@opslira.local"),
		AdminName:        envOr("ADMIN_NAME", "Opslira Super Admin"),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
	}

	var err error
	if cfg.SessionTTL, err = secondsEnv("SESSION_TTL_SECONDS", 86400); err != nil {
		return Config{}, err
	}
	if cfg.Redis.DB, err = intEnv("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if cfg.Redis.CacheDB, err = intEnv("REDIS_CACHE_DB", 1); err != nil {
		return Config{}, err
	}
	if cfg.ProbeConcurrency, err = intEnv("PROBE_CONCURRENCY", 8); err != nil {
		return Config{}, err
	}
	if cfg.TickSeconds, err = intEnv("RUNNER_TICK_SECONDS", 15); err != nil {
		return Config{}, err
	}
	if cfg.ReportTickSeconds, err = intEnv("REPORT_TICK_SECONDS", 60); err != nil {
		return Config{}, err
	}
	if cfg.BeszelTokenCacheTTL, err = secondsEnv("BESZEL_TOKEN_CACHE_TTL", 43200); err != nil {
		return Config{}, err
	}
	if cfg.ProbeConcurrency < 1 {
		return Config{}, fmt.Errorf("PROBE_CONCURRENCY must be at least 1, got %d", cfg.ProbeConcurrency)
	}
	if cfg.TickSeconds < 1 || cfg.ReportTickSeconds < 1 {
		return Config{}, fmt.Errorf("tick intervals must be at least 1 second")
	}
	if cfg.SessionTTL < time.Second {
		return Config{}, fmt.Errorf("SESSION_TTL_SECONDS must be at least 1 second")
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
	if cfg.Mail.Port != "" {
		if _, err := strconv.Atoi(cfg.Mail.Port); err != nil {
			return Config{}, fmt.Errorf("MAIL_PORT must be an integer, got %q", cfg.Mail.Port)
		}
	}

	return cfg, nil
}

// Location resolves the configured timezone for scheduler and report math.
func (c Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
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

func secondsEnv(key string, fallback int) (time.Duration, error) {
	seconds, err := intEnv(key, fallback)
	if err != nil {
		return 0, err
	}

	return time.Duration(seconds) * time.Second, nil
}

func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}

	return out
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
