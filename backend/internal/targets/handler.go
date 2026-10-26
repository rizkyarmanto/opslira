package targets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/auth"
	"opslira/backend/internal/credentials"
	"opslira/backend/internal/httpx"
	"opslira/backend/internal/model"
	"opslira/backend/internal/probes"
	"opslira/backend/internal/probes/adapters"
	"opslira/backend/internal/probestate"
	"opslira/backend/internal/uuid"
)

// Handler serves the target CRUD, search, test-connection and maintenance
// endpoints under /api/v1/targets.
type Handler struct {
	pool   *pgxpool.Pool
	rdb    *goredis.Client // DB0: probe state (shown on the detail page)
	cache  *goredis.Client // DB1: beszel token cache (Laravel Cache facade)
	enc    *credentials.Service
	probes *probes.Service
}

// NewHandler wires the targets endpoints. cache must be the DB1 client: the
// Beszel token cache lives in the cache database like Laravel's Redis store.
func NewHandler(pool *pgxpool.Pool, rdb, cache *goredis.Client, enc *credentials.Service, probeSvc *probes.Service) *Handler {
	return &Handler{pool: pool, rdb: rdb, cache: cache, enc: enc, probes: probeSvc}
}

// Mount registers the /targets routes behind the session middleware.
func (h *Handler) Mount(r chi.Router, mw func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(mw)
		r.Get("/targets", h.index)
		r.Get("/targets/search", h.search)
		r.Post("/targets", h.store)
		r.Post("/targets/test-connection", h.testConnection)
		r.Get("/targets/{targetID}", h.show)
		r.Put("/targets/{targetID}", h.update)
		r.Delete("/targets/{targetID}", h.destroy)
		r.Post("/targets/{targetID}/enable", h.enable)
		r.Post("/targets/{targetID}/toggle-maintenance", h.toggleMaintenance)
	})
}

// index lists targets with the filter set from TargetController::index:
// active by default, maintenance rows hidden unless asked for.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	where := []string{}
	args := []any{}

	eq := func(column, key string) {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			args = append(args, value)
			where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	eq("dashboard_group", "dashboard_group")
	eq("target_type", "target_type")
	eq("environment", "environment")
	eq("current_status", "status")

	switch state := query.Get("state"); state {
	case "disabled":
		where = append(where, "is_active = false")
	case "all":
	default:
		where = append(where, "is_active = true")
	}

	switch maintenance := query.Get("maintenance"); maintenance {
	case "yes":
		where = append(where, "is_maintenance = true")
	case "all":
	default:
		where = append(where, "is_maintenance = false")
	}

	sqlText := "SELECT " + model.Columns + " FROM monitoring_targets"
	if len(where) > 0 {
		sqlText += " WHERE " + strings.Join(where, " AND ")
	}
	sqlText += " ORDER BY name"

	rows, err := h.pool.Query(r.Context(), sqlText, args...)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load targets.")
		return
	}
	defer rows.Close()

	targets := []model.Target{}
	for rows.Next() {
		t, err := model.ScanTarget(rows)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to load targets.")
			return
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load targets.")
		return
	}

	// Laravel only() keeps the query keys that were actually sent.
	filters := map[string]any{}
	for _, key := range []string{"dashboard_group", "target_type", "environment", "status", "state", "maintenance"} {
		if _, present := query[key]; present {
			filters[key] = query.Get(key)
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"targets": targets, "filters": filters})
}

// searchResult is the command-palette projection (subset of columns).
type searchResult struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	HostOrEndpoint string `json:"host_or_endpoint"`
	Port           *int   `json:"port"`
	DashboardGroup string `json:"dashboard_group"`
	TargetType     string `json:"target_type"`
	ProtocolType   string `json:"protocol_type"`
	CurrentStatus  string `json:"current_status"`
	IsMaintenance  bool   `json:"is_maintenance"`
}

// search matches name/endpoint/type/protocol/environment with prefix-first
// ranking, capped at 12 active targets.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	escaped := escapeLike(term)
	like := "%" + escaped + "%"
	prefix := escaped + "%"

	where := []string{"is_active = true"}
	args := []any{}
	if term != "" {
		args = append(args, like)
		likeParam := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(
			"(name ILIKE %[1]s OR host_or_endpoint ILIKE %[1]s OR target_type ILIKE %[1]s OR protocol_type ILIKE %[1]s OR environment ILIKE %[1]s)",
			likeParam))
	}
	args = append(args, prefix)
	prefixParam := fmt.Sprintf("$%d", len(args))

	sqlText := `SELECT id, name, host_or_endpoint, port, dashboard_group, target_type, protocol_type,
	       current_status, is_maintenance
	  FROM monitoring_targets
	 WHERE ` + strings.Join(where, " AND ") + `
	 ORDER BY CASE WHEN name ILIKE ` + prefixParam + ` THEN 0
	               WHEN host_or_endpoint ILIKE ` + prefixParam + ` THEN 1
	               ELSE 2 END,
	          name
	 LIMIT 12`

	rows, err := h.pool.Query(r.Context(), sqlText, args...)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Search failed.")
		return
	}
	defer rows.Close()

	data := []searchResult{}
	for rows.Next() {
		var (
			sr   searchResult
			port sql.NullInt32
		)
		if err := rows.Scan(&sr.ID, &sr.Name, &sr.HostOrEndpoint, &port, &sr.DashboardGroup,
			&sr.TargetType, &sr.ProtocolType, &sr.CurrentStatus, &sr.IsMaintenance); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Search failed.")
			return
		}
		sr.Port = intPtr(port)
		data = append(data, sr)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"data": data})
}

// store validates, normalizes and creates a target plus its credentials.
func (h *Handler) store(w http.ResponseWriter, r *http.Request) {
	var payload TargetPayload
	if httpx.DecodeOr422(w, r, &payload) {
		return
	}
	if errs := validate(payload); len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}
	normalize(&payload)

	id := uuid.New()
	var createdBy *string
	if actor, ok := auth.UserFrom(r.Context()); ok {
		createdBy = &actor.ID
	}

	checkInterval := 60
	if payload.CheckIntervalSeconds != nil {
		checkInterval = *payload.CheckIntervalSeconds
	}
	timeout := 5
	if payload.TimeoutSeconds != nil {
		timeout = *payload.TimeoutSeconds
	}
	retry := 3
	if payload.RetryThreshold != nil {
		retry = *payload.RetryThreshold
	}
	degraded := 1000
	if payload.DegradedThresholdMs != nil {
		degraded = *payload.DegradedThresholdMs
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO monitoring_targets
			(id, name, dashboard_group, target_type, environment, host_or_endpoint, port, protocol_type,
			 config_payload, check_interval_seconds, timeout_seconds, retry_threshold, degraded_threshold_ms,
			 current_status, is_maintenance, is_active, next_check_at, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'PENDING',false,true,now(),$14,now(),now())`,
		id, payload.Name, payload.DashboardGroup, payload.TargetType, payload.Environment,
		payload.HostOrEndpoint, payload.Port, payload.ProtocolType, encodeObject(payload.ConfigPayload),
		checkInterval, timeout, retry, degraded, createdBy,
	)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to create target.")
		return
	}

	if err := h.persistCredentials(r.Context(), id, payload.Credentials); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to store credentials.")
		return
	}

	created, err := loadTarget(r.Context(), h.pool, id)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load target.")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"target": created})
}

// show returns the target with its Redis state, credential flag, last 50
// logs and last 20 incidents.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "targetID")
	if !uuid.Valid(targetID) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}

	target, err := loadTarget(r.Context(), h.pool, targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load target.")
		return
	}

	state, stateErr := probestate.Get(r.Context(), h.rdb, targetID)
	if stateErr != nil {
		// Redis unavailable: render the same default the state service uses
		// instead of failing the detail page.
		state = model.TargetState{Status: model.StatusPending, SummaryMetrics: map[string]any{}}
	}
	target.State = &state

	var hasCredentials bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM target_credentials WHERE target_id = $1)`, targetID,
	).Scan(&hasCredentials); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load target.")
		return
	}
	target.HasCredentials = &hasCredentials

	logs, err := scanLogs(r.Context(), h.pool, targetID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load logs.")
		return
	}

	incidents, err := h.loadIncidents(r.Context(), targetID, 20)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load incidents.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"target":    target,
		"logs":      logs,
		"incidents": incidents,
	})
}

// update applies a partial or full payload; credentials only change when the
// key is present (mirroring array_key_exists in TargetController::update).
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "targetID")
	if !uuid.Valid(targetID) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}

	existing, err := loadTarget(r.Context(), h.pool, targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load target.")
		return
	}

	var payload TargetPayload
	present, ok := httpx.DecodeWithPresence(w, r, &payload)
	if !ok {
		return
	}
	if errs := validate(payload); len(errs) > 0 {
		httpx.Invalid(w, errs)
		return
	}
	normalize(&payload)

	// Beszel hub/token invalidation when the hub URL or login changed.
	if existing.TargetType == "BESZEL_SERVER" {
		oldHub, _ := existing.ConfigPayload["hub_url"].(string)
		if oldHub == "" {
			oldHub, _ = existing.ConfigPayload["host"].(string)
		}
		oldEmail := h.beszelEmail(r.Context(), existing)
		newHub := ""
		if payload.ConfigPayload != nil {
			newHub, _ = payload.ConfigPayload["hub_url"].(string)
			if newHub == "" {
				newHub, _ = payload.ConfigPayload["host"].(string)
			}
		}
		newEmail := oldEmail
		if payload.Credentials != nil {
			if email, ok := payload.Credentials.Payload["email"].(string); ok {
				newEmail = email
			}
		}
		if oldHub != "" && oldEmail != "" {
			adapters.ForgetToken(r.Context(), probes.NewRedisCache(h.cache), oldHub, oldEmail)
		}
		if newHub != "" && newEmail != "" && (newHub != oldHub || payload.Credentials != nil) {
			adapters.ForgetToken(r.Context(), probes.NewRedisCache(h.cache), newHub, newEmail)
		}
	}

	set := []string{}
	args := []any{targetID}
	put := func(column, key string, value any) {
		if _, ok := present[key]; !ok {
			return
		}
		args = append(args, value)
		set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	put("name", "name", payload.Name)
	put("dashboard_group", "dashboard_group", payload.DashboardGroup)
	put("target_type", "target_type", payload.TargetType)
	put("environment", "environment", payload.Environment)
	put("host_or_endpoint", "host_or_endpoint", payload.HostOrEndpoint)
	put("port", "port", payload.Port)
	put("protocol_type", "protocol_type", payload.ProtocolType)
	put("config_payload", "config_payload", encodeObject(payload.ConfigPayload))
	put("check_interval_seconds", "check_interval_seconds", payload.CheckIntervalSeconds)
	put("timeout_seconds", "timeout_seconds", payload.TimeoutSeconds)
	put("retry_threshold", "retry_threshold", payload.RetryThreshold)
	put("degraded_threshold_ms", "degraded_threshold_ms", payload.DegradedThresholdMs)

	if len(set) > 0 {
		sqlText := "UPDATE monitoring_targets SET " + strings.Join(set, ", ") + ", updated_at = now() WHERE id = $1"
		if _, err := h.pool.Exec(r.Context(), sqlText, args...); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to update target.")
			return
		}
	}

	if payload.Credentials != nil {
		if err := h.persistCredentials(r.Context(), targetID, payload.Credentials); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Failed to store credentials.")
			return
		}
	}

	updated, err := loadTarget(r.Context(), h.pool, targetID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to load target.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"target": updated})
}

// destroy soft-disables: DELETE sets is_active=false, never a hard delete.
func (h *Handler) destroy(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "targetID")
	if !uuid.Valid(targetID) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE monitoring_targets SET is_active = false, updated_at = now() WHERE id = $1`, targetID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to disable target.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	httpx.Ok(w)
}

// enable re-activates a soft-disabled target.
func (h *Handler) enable(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "targetID")
	if !uuid.Valid(targetID) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE monitoring_targets SET is_active = true, updated_at = now() WHERE id = $1`, targetID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to enable target.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "is_active": true})
}

// toggleMaintenance pauses/resumes a target. On the shared UPDATE the CASE
// sees the OLD is_maintenance, so enabling maps to MAINTENANCE and disabling
// resets to PENDING (the next probe decides the real status).
func (h *Handler) toggleMaintenance(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "targetID")
	if !uuid.Valid(targetID) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}

	var (
		maintenance bool
		status      string
	)
	err := h.pool.QueryRow(r.Context(),
		`UPDATE monitoring_targets
		    SET is_maintenance = NOT is_maintenance,
		        current_status = CASE WHEN NOT is_maintenance THEN 'MAINTENANCE' ELSE 'PENDING' END,
		        updated_at = now()
		  WHERE id = $1
		 RETURNING is_maintenance, current_status`, targetID,
	).Scan(&maintenance, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(w, http.StatusNotFound, "Target not found.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Failed to update target.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"is_maintenance": maintenance,
		"current_status": status,
	})
}

// testConnection delegates to the probes package's pre-save handler: it runs
// the adapter without persisting a log, maps ErrUnknownProtocol onto the
// Laravel 422 body, and always answers the camelCase ProbeResult shape
// (TargetController::testConnectionPayload ported).
func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	h.probes.TestConnectionHandler()(w, r)
}

func (h *Handler) loadIncidents(ctx context.Context, targetID string, limit int) ([]model.Incident, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, target_id, severity, status, root_cause_message, started_at, resolved_at, duration_seconds
		   FROM incidents
		  WHERE target_id = $1
		  ORDER BY started_at DESC
		  LIMIT $2`, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Incident{}
	for rows.Next() {
		var (
			incident model.Incident
			resolved sql.NullTime
			duration sql.NullInt32
		)
		if err := rows.Scan(&incident.ID, &incident.TargetID, &incident.Severity, &incident.Status,
			&incident.RootCauseMessage, &incident.StartedAt, &resolved, &duration); err != nil {
			return nil, err
		}
		incident.ResolvedAt = timePtr(resolved)
		incident.DurationSeconds = intPtr(duration)
		out = append(out, incident)
	}

	return out, rows.Err()
}

// persistCredentials mirrors TargetController::persistCredentials: NONE or an
// empty auth type deletes the row, anything else upserts ciphertext+IV+tag.
func (h *Handler) persistCredentials(ctx context.Context, targetID string, input *CredentialsInput) error {
	if input == nil {
		return nil
	}
	if input.AuthType == "" || input.AuthType == "NONE" {
		_, err := h.pool.Exec(ctx, `DELETE FROM target_credentials WHERE target_id = $1`, targetID)
		return err
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	encrypted, err := h.enc.Encrypt(payload)
	if err != nil {
		return err
	}

	_, err = h.pool.Exec(ctx,
		`INSERT INTO target_credentials (id, target_id, auth_type, encrypted_payload, encryption_iv, encryption_tag, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,now())
		 ON CONFLICT (target_id) DO UPDATE
		   SET auth_type = EXCLUDED.auth_type,
		       encrypted_payload = EXCLUDED.encrypted_payload,
		       encryption_iv = EXCLUDED.encryption_iv,
		       encryption_tag = EXCLUDED.encryption_tag,
		       updated_at = now()`,
		uuid.New(), targetID, input.AuthType, encrypted.Payload, encrypted.IV, encrypted.Tag)

	return err
}

// beszelEmail decrypts stored credentials to find the cached-token email key.
func (h *Handler) beszelEmail(ctx context.Context, target model.Target) string {
	var payload, iv, tag string
	err := h.pool.QueryRow(ctx,
		`SELECT encrypted_payload, encryption_iv, encryption_tag FROM target_credentials WHERE target_id = $1`,
		target.ID).Scan(&payload, &iv, &tag)
	if err != nil {
		return ""
	}
	decrypted, err := h.enc.Decrypt(payload, iv, tag)
	if err != nil {
		return ""
	}
	email, _ := decrypted["email"].(string)

	return email
}

// escapeLike mirrors the PHP search escaping: only % and _ are wildcards in
// ILIKE, and the query wraps the term in %...%.
func escapeLike(term string) string {
	return strings.NewReplacer("%", `\%`, "_", `\_`).Replace(term)
}
