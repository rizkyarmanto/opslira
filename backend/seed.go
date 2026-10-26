// Package opslira holds the root-level database bootstrap: the embedded SQL
// migrations and the idempotent seeders ported from DatabaseSeeder
// (users + targets + schedules) and ProductionSeeder (superadmin only).
package opslira

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/auth"
	"opslira/backend/internal/config"
	"opslira/backend/internal/uuid"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrations returns the embedded SQL files in filename order (the numeric
// prefixes keep Laravel's chronological order).
func Migrations() ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	return names, nil
}

// ApplyMigrations runs every embedded SQL file that has not run yet, tracked
// in schema_migrations (name + checksum). Every file must be idempotent so a
// partially migrated legacy schema (the old Laravel tables) converges: the
// statements use IF NOT EXISTS / DROP CONSTRAINT IF EXISTS shapes.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) PRIMARY KEY,
		checksum VARCHAR(64) NOT NULL,
		ran_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return 0, fmt.Errorf("schema_migrations: %w", err)
	}

	names, err := Migrations()
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, name := range names {
		raw, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return applied, fmt.Errorf("read %s: %w", name, err)
		}

		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&exists); err != nil {
			return applied, fmt.Errorf("migration lookup: %w", err)
		}
		if exists {
			continue
		}

		// Whole-file execution: statements are plain SQL separated by ';'
		// and contain no dollar-quoted bodies except the partition DO block,
		// which pgx sends as one multi-statement query safely.
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			return applied, fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`,
			name, fmt.Sprintf("%x", len(raw))); err != nil {
			return applied, fmt.Errorf("record %s: %w", name, err)
		}
		applied++
	}

	return applied, nil
}

// DefaultPassword is the shared dev password for the seeded accounts
// (UserSeeder's ChangeMe!2026).
const DefaultPassword = "ChangeMe!2026"

// SeedAll runs the local development seed: four users, eight targets
// (all types, interval 60) and three schedules (DAILY/WEEKLY/MONTHLY,
// Asia/Jakarta, next_run_at = now + 1 day). It is idempotent
// (update-or-create per natural key, like the Laravel seeders).
func SeedAll(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	users := []struct {
		name  string
		email string
		role  string
	}{
		{"Opslira Super Admin", "admin@opslira.local", "SUPER_ADMIN"},
		{"Network SysAdmin", "sysadmin@opslira.local", "SYSADMIN"},
		{"Application Developer", "developer@opslira.local", "DEVELOPER"},
		{"IT Manager", "manager@opslira.local", "MANAGER"},
	}
	for _, user := range users {
		if err := upsertUser(ctx, pool, user.name, user.email, user.role, DefaultPassword); err != nil {
			return err
		}
	}

	if err := seedTargets(ctx, pool); err != nil {
		return err
	}

	return seedSchedules(ctx, pool, cfg)
}

// SeedProduction runs ProductionSeeder: superadmin only, from ADMIN_* env.
func SeedProduction(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	if cfg.AdminPassword == "" {
		return fmt.Errorf("ADMIN_PASSWORD is empty. Set it in the environment, then rerun the seeder.")
	}

	return upsertUser(ctx, pool, cfg.AdminName, cfg.AdminEmail, "SUPER_ADMIN", cfg.AdminPassword)
}

func upsertUser(ctx context.Context, pool *pgxpool.Pool, name, email, role, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, name, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, true, now(), now())
		 ON CONFLICT (email) DO UPDATE
		   SET name = EXCLUDED.name,
		       password_hash = EXCLUDED.password_hash,
		       role = EXCLUDED.role,
		       is_active = true,
		       updated_at = now()`,
		uuid.New(), name, email, hash, role)
	if err != nil {
		return fmt.Errorf("seed user %s: %w", email, err)
	}

	return nil
}

// seededTarget mirrors MonitoringTargetSeeder rows (eight targets covering
// every target_type, all with check_interval_seconds = 60).
type seededTarget struct {
	name          string
	group         string
	targetType    string
	environment   string
	host          string
	port          *int
	protocol      string
	configPayload map[string]any
}

func seedTargets(ctx context.Context, pool *pgxpool.Pool) error {
	targets := []seededTarget{
		{
			name: "Office Core Router", group: "NETWORK", targetType: "NETWORK_DEVICE",
			environment: "INTERNAL", host: "192.168.1.1", protocol: "ICMP_SNMP",
			configPayload: map[string]any{
				"sub_type": "ROUTER", "icmp_count": 4, "enable_snmp": true,
				"snmp_version": "2c", "interfaces": []any{"ether1-wan", "bridge-lan"},
			},
		},
		{
			name: "ERP Portal Production", group: "APP_DOMAIN_DB", targetType: "WEB_APP",
			environment: "PRODUCTION", host: "https://erp.kantor.co.id/health", protocol: "HTTP_HEALTH",
			configPayload: map[string]any{
				"method": "GET", "expected_status": 200, "verify_ssl": true,
				"expected_json_key": "status", "expected_json_value": "ok",
			},
		},
		{
			name: "kantor.co.id Domain", group: "APP_DOMAIN_DB", targetType: "DOMAIN",
			environment: "PRODUCTION", host: "kantor.co.id", protocol: "WHOIS_SSL",
			configPayload: map[string]any{
				"whois_server": "whois.id", "check_ssl_port": 443,
				"expected_a_records": []any{}, "warn_days_before": 14,
			},
		},
		{
			name: "ERP Database Production", group: "APP_DOMAIN_DB", targetType: "DATABASE",
			environment: "PRODUCTION", host: "10.10.0.20", port: intPtr(5432),
			protocol: "DB_NATIVE_PING",
			configPayload: map[string]any{
				"db_driver": "pgsql", "database_name": "erp_prod",
				"query": "SELECT 1", "collect_connection_stats": true,
			},
		},
		{
			name: "Office Mail Server", group: "MAIL_S3_BESZEL", targetType: "MAIL_SERVER",
			environment: "PRODUCTION", host: "mail.kantor.co.id", protocol: "SMTP_DNSBL",
			configPayload: map[string]any{
				"check_ports":     map[string]any{"smtp": 587, "imap": 993},
				"check_dnsbl":     true,
				"dnsbl_providers": []any{"zen.spamhaus.org", "bl.spamcop.net"},
			},
		},
		{
			name: "Backup Bucket Production", group: "MAIL_S3_BESZEL", targetType: "S3_STORAGE",
			environment: "PRODUCTION", host: "https://s3.ap-southeast-1.amazonaws.com",
			protocol: "S3_HEAD",
			configPayload: map[string]any{
				"region": "ap-southeast-1", "bucket_name": "kantor-backup-prod",
				"use_path_style_endpoint": true,
			},
		},
		{
			name: "Beszel Hub Node-01", group: "MAIL_S3_BESZEL", targetType: "BESZEL_SERVER",
			environment: "PRODUCTION", host: "http://beszel.kantor.co.id:8090",
			protocol: "BESZEL_API",
			configPayload: map[string]any{
				"beszel_system_name": "Node-01",
				"warn_cpu_pct":       85, "warn_mem_pct": 90, "warn_disk_pct": 85,
			},
		},
		{
			// icmp_only is written explicitly because the seeder bypasses the
			// controller, and without it the target inherits TCP fallback.
			name: "Internet DNS (8.8.8.8)", group: "NETWORK", targetType: "INTERNET",
			environment: "PRODUCTION", host: "8.8.8.8", protocol: "ICMP_PING",
			configPayload: map[string]any{"icmp_count": 4, "icmp_only": true},
		},
	}

	for _, target := range targets {
		configJSON, err := json.Marshal(target.configPayload)
		if err != nil {
			return fmt.Errorf("seed target %s config: %w", target.name, err)
		}
		// No unique constraint on name (legacy schema), so upsert manually:
		// update by name when present, insert otherwise.
		var existingID string
		err = pool.QueryRow(ctx,
			`SELECT id FROM monitoring_targets WHERE name = $1 LIMIT 1`, target.name).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("seed target %s lookup: %w", target.name, err)
		}
		if existingID != "" {
			_, err = pool.Exec(ctx,
				`UPDATE monitoring_targets SET
				   dashboard_group = $2, target_type = $3, environment = $4,
				   host_or_endpoint = $5, port = $6, protocol_type = $7,
				   config_payload = $8, check_interval_seconds = 60, updated_at = now()
				 WHERE id = $1`,
				existingID, target.group, target.targetType, target.environment,
				target.host, target.port, target.protocol, configJSON)
		} else {
			_, err = pool.Exec(ctx,
				`INSERT INTO monitoring_targets
					(id, name, dashboard_group, target_type, environment, host_or_endpoint, port,
					 protocol_type, config_payload, check_interval_seconds, timeout_seconds,
					 retry_threshold, degraded_threshold_ms, current_status, is_maintenance,
					 is_active, next_check_at, created_at, updated_at)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,60,5,3,1000,'PENDING',false,true,now(),now(),now())`,
				uuid.New(), target.name, target.group, target.targetType, target.environment,
				target.host, target.port, target.protocol, configJSON)
		}
		if err != nil {
			return fmt.Errorf("seed target %s: %w", target.name, err)
		}
	}

	return nil
}

// seedSchedules ports ReportScheduleSeeder: DAILY/WEEKLY/MONTHLY with the
// seed cron expressions, Asia/Jakarta, next_run_at now+1d.
func seedSchedules(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	schedules := []struct {
		name       string
		periodType string
		cron       string
		formats    []string
	}{
		{"Daily Operations Report", "DAILY", "0 8 * * *", []string{"HTML", "PDF", "XLSX"}},
		{"Weekly Performance Report", "WEEKLY", "0 8 * * 1", []string{"PDF", "XLSX"}},
		{"Monthly Executive SLA Report", "MONTHLY", "0 8 1 * *", []string{"PDF", "XLSX"}},
	}

	scope, err := json.Marshal(map[string]any{
		"dashboard_groups": []string{"ALL"},
		"environments":     []string{"ALL"},
	})
	if err != nil {
		return err
	}
	nextRun := time.Now().Add(24 * time.Hour)

	for _, schedule := range schedules {
		formatsJSON, err := json.Marshal(schedule.formats)
		if err != nil {
			return fmt.Errorf("seed schedule %s formats: %w", schedule.name, err)
		}
		// No unique constraint on name (legacy schema): update by name, insert otherwise.
		var existingID string
		err = pool.QueryRow(ctx,
			`SELECT id FROM report_schedules WHERE name = $1 LIMIT 1`, schedule.name).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("seed schedule %s lookup: %w", schedule.name, err)
		}
		if existingID != "" {
			_, err = pool.Exec(ctx,
				`UPDATE report_schedules SET
				   period_type = $2, cron_expression = $3, timezone = 'Asia/Jakarta',
				   report_formats = $4, next_run_at = $5, updated_at = now()
				 WHERE id = $1`,
				existingID, schedule.periodType, schedule.cron, formatsJSON, nextRun)
		} else {
			_, err = pool.Exec(ctx,
				`INSERT INTO report_schedules
					(id, name, period_type, cron_expression, timezone, scope_filter,
					 report_formats, recipients_to, recipients_cc, send_only_on_issues,
					 is_active, next_run_at, created_at, updated_at)
				 VALUES ($1,$2,$3,$4,'Asia/Jakarta',$5,$6,'[]'::jsonb,'[]'::jsonb,false,true,$7,now(),now())`,
				uuid.New(), schedule.name, schedule.periodType, schedule.cron, scope, formatsJSON, nextRun)
		}
		if err != nil {
			return fmt.Errorf("seed schedule %s: %w", schedule.name, err)
		}
	}

	return nil
}

func intPtr(value int) *int { return &value }
