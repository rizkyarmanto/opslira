package probes

import (
	"context"
	"crypto/md5" //nolint:gosec // cache key only, matches PHP md5()
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/credentials"
	"opslira/backend/internal/model"
	"opslira/backend/internal/probestate"
)

// Hooks are the incident/notification callbacks wired from internal/incidents
// by cmd/api; every field may be nil (tests, or alerts not configured).
type Hooks struct {
	// OpenIncident creates the OPEN CRITICAL incident and returns its id
	// (incidents.Service.Open).
	OpenIncident func(ctx context.Context, t model.Target, severity, rootCause string) (string, error)
	// ResolveIncidents closes every open incident for the target
	// (incidents.Service.ResolveOpen).
	ResolveIncidents func(ctx context.Context, targetID string) error
	// Notify sends the state-change alert inline
	// (incidents.Service.Notify; QUEUE_CONNECTION=sync in PHP).
	Notify func(ctx context.Context, t model.Target, event, incidentID string)
}

// Service is the probe pipeline facade used by the scheduler tick and the
// targets test-connection route: claim due targets, run them through the
// adapter factory, write monitoring_logs + Redis state, and react to state
// changes (ProbeExecutionService + RunnerController::claim in one process).
type Service struct {
	pool    *pgxpool.Pool
	exec    *ExecutionService
	factory Factory
	logger  *slog.Logger
}

// NewService wires the pipeline. creds backs the AES-256-GCM decryption
// (credentials.Service satisfies CredentialDecryptor), rdb is the DB0 data
// client behind target state, and factory resolves protocol_type to adapters
// (adapters.NewFactory; passed in because the adapters subpackage imports
// this one for the Adapter contract).
func NewService(pool *pgxpool.Pool, rdb goredis.Cmdable, creds *credentials.Service, factory Factory, hooks Hooks, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	exec := &ExecutionService{
		Factory:   factory,
		State:     probestate.New(rdb),
		Decryptor: creds,
		Incidents: &hookStore{pool: pool, hooks: hooks},
		Alert:     hookNotifier(hooks),
		Log:       logger,
	}
	return &Service{pool: pool, exec: exec, factory: factory, logger: logger}
}

// RunDue claims up to 2x concurrency due targets (is_active AND NOT
// is_maintenance AND next_check_at <= now), advances their next_check_at, and
// executes them concurrently. Returns how many probes ran. The scheduler
// calls this once per RUNNER_TICK_SECONDS; its overlap guard keeps two pools
// from racing.
func (s *Service) RunDue(ctx context.Context, concurrency int) (int, error) {
	if concurrency < 1 {
		concurrency = 1
	}
	jobs, err := claim(ctx, s.pool, concurrency*2)
	if err != nil {
		return 0, err
	}
	if len(jobs) == 0 {
		return 0, nil
	}

	var wg sync.WaitGroup
	results := make(chan error, len(jobs))

	for _, job := range jobs {
		wg.Add(1)
		go func(job TargetJob) {
			defer wg.Done()
			_, err := s.exec.Execute(ctx, job.Target, job.Credential)
			results <- err
		}(job)
	}
	wg.Wait()
	close(results)

	executed := 0
	for err := range results {
		if err != nil {
			s.logger.ErrorContext(ctx, "probe execution failed", "err", err)
			continue
		}
		executed++
	}
	return executed, nil
}

// TestConnectionHandler is the pre-save connection test the targets router
// mounts at POST /api/v1/targets/test-connection.
func (s *Service) TestConnectionHandler() http.HandlerFunc {
	return testConnectionHandler(s)
}

// ForgetBeszelToken drops the cached PocketBase login JWT for hub+email. The
// targets update flow calls it when a BESZEL_SERVER target's hub URL or email
// changes, so the next probe re-authenticates instead of using the stale
// token (PHP BeszelApiProbeAdapter::forgetToken).
func ForgetBeszelToken(ctx context.Context, rdb goredis.Cmdable, hubURL, email string) {
	if rdb == nil {
		return
	}
	NewRedisCache(rdb).Del(ctx, BeszelTokenCacheKey(hubURL, email))
}

// BeszelTokenCacheKey mirrors the PHP cache key
// beszel:token:<md5(hub_url)>:<md5(email)>; the beszel adapter caches its
// login JWT under it, and the targets update flow deletes it.
func BeszelTokenCacheKey(hubURL, email string) string {
	sumHub := md5.Sum([]byte(hubURL))  //nolint:gosec // cache key only, matches PHP md5()
	sumEmail := md5.Sum([]byte(email)) //nolint:gosec // cache key only, matches PHP md5()
	return "beszel:token:" + hex.EncodeToString(sumHub[:]) + ":" + hex.EncodeToString(sumEmail[:])
}

// TargetJob is one claimed due target with its joined credentials row.
type TargetJob struct {
	Target     model.Target
	Credential *Credential
}

// claim hands out due targets and pushes their next_check_at forward inside
// one transaction; the row locks are what keep two processes from probing the
// same target twice (RunnerController::claim). The cost is explicit: a probe
// that dies mid-flight leaves next_check_at already pushed forward, so that
// target waits one interval before it is tried again.
func claim(ctx context.Context, pool *pgxpool.Pool, limit int) ([]TargetJob, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT t.id, t.name, t.dashboard_group, t.target_type, t.environment,
		       t.host_or_endpoint, t.port, t.protocol_type, t.config_payload,
		       t.check_interval_seconds, t.timeout_seconds, t.retry_threshold,
		       t.degraded_threshold_ms, t.current_status, t.is_maintenance,
		       t.is_active, t.last_checked_at, t.next_check_at,
		       c.encrypted_payload, c.encryption_iv, c.encryption_tag
		FROM monitoring_targets t
		LEFT JOIN target_credentials c ON c.target_id = t.id
		WHERE t.is_active AND NOT t.is_maintenance AND t.next_check_at <= NOW()
		ORDER BY t.next_check_at
		LIMIT $1
		FOR UPDATE OF t`,
		limit,
	)
	if err != nil {
		return nil, err
	}

	var jobs []TargetJob
	var ids []string
	for rows.Next() {
		var job TargetJob
		var configPayload []byte
		var port sql.NullInt32
		var lastCheckedAt sql.NullTime
		var credentialPayload, credentialIV, credentialTag sql.NullString
		err := rows.Scan(
			&job.Target.ID, &job.Target.Name, &job.Target.DashboardGroup,
			&job.Target.TargetType, &job.Target.Environment,
			&job.Target.HostOrEndpoint, &port, &job.Target.ProtocolType,
			&configPayload,
			&job.Target.CheckIntervalSeconds, &job.Target.TimeoutSeconds,
			&job.Target.RetryThreshold, &job.Target.DegradedThresholdMs,
			&job.Target.CurrentStatus, &job.Target.IsMaintenance,
			&job.Target.IsActive, &lastCheckedAt, &job.Target.NextCheckAt,
			&credentialPayload, &credentialIV, &credentialTag,
		)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if port.Valid {
			value := int(port.Int32)
			job.Target.Port = &value
		}
		if lastCheckedAt.Valid {
			value := lastCheckedAt.Time
			job.Target.LastCheckedAt = &value
		}
		if len(configPayload) > 0 {
			if err := json.Unmarshal(configPayload, &job.Target.ConfigPayload); err != nil {
				rows.Close()
				return nil, fmt.Errorf("target %s: config_payload: %w", job.Target.ID, err)
			}
		}
		if credentialPayload.Valid {
			job.Credential = &Credential{
				EncryptedPayload: credentialPayload.String,
				EncryptionIV:     credentialIV.String,
				EncryptionTag:    credentialTag.String,
			}
		}
		jobs = append(jobs, job)
		ids = append(ids, job.Target.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE monitoring_targets
			SET next_check_at = NOW() + make_interval(secs => check_interval_seconds)
			WHERE id = ANY($1)`,
			ids,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return jobs, nil
}

// hookStore satisfies IncidentStore: row-level writes (monitoring_targets,
// monitoring_logs) always run through the pool; incident rows and alerts go
// through the Hooks when wired.
type hookStore struct {
	pool  *pgxpool.Pool
	hooks Hooks
}

func (h *hookStore) InsertLog(ctx context.Context, targetID string, result ProbeResult, checkedAt time.Time) error {
	metrics, err := json.Marshal(result.MetricsPayload)
	if err != nil || len(metrics) == 0 {
		metrics = []byte("{}")
	}
	_, err = h.pool.Exec(ctx, `
		INSERT INTO monitoring_logs
			(target_id, status, response_time_ms, http_status_code, error_message, metrics_payload, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		targetID, result.Status, result.ResponseTimeMs, result.HTTPStatusCode, result.ErrorMessage, metrics, checkedAt,
	)
	return err
}

func (h *hookStore) UpdateTargetStatus(ctx context.Context, t model.Target, status string, checkedAt time.Time) error {
	_, err := h.pool.Exec(ctx, `
		UPDATE monitoring_targets
		SET current_status = $2, last_checked_at = $3, updated_at = NOW()
		WHERE id = $1`,
		t.ID, status, checkedAt,
	)
	return err
}

func (h *hookStore) TouchTarget(ctx context.Context, t model.Target, checkedAt time.Time) error {
	_, err := h.pool.Exec(ctx, `
		UPDATE monitoring_targets
		SET last_checked_at = $2, updated_at = NOW()
		WHERE id = $1`,
		t.ID, checkedAt,
	)
	return err
}

func (h *hookStore) OpenIncident(ctx context.Context, t model.Target, rootCause string, _ time.Time) (string, error) {
	if h.hooks.OpenIncident == nil {
		return "", nil
	}
	// The incidents service timestamps started_at internally (NOW()).
	return h.hooks.OpenIncident(ctx, t, "CRITICAL", rootCause)
}

func (h *hookStore) ResolveOpenIncidents(ctx context.Context, t model.Target, _ time.Time) (int, error) {
	if h.hooks.ResolveIncidents == nil {
		return 0, nil
	}
	// The incidents service stamps resolved_at/duration internally.
	return 0, h.hooks.ResolveIncidents(ctx, t.ID)
}

// hookNotifier adapts the Notify hook to the inline StateChangeHook.
func hookNotifier(hooks Hooks) StateChangeHook {
	if hooks.Notify == nil {
		return nil
	}
	return func(ctx context.Context, t model.Target, event, incidentID string) {
		hooks.Notify(ctx, t, event, incidentID)
	}
}
