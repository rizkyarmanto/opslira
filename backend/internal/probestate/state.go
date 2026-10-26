// Package probestate ports TargetStateService: the Redis snapshot behind
// GET /dashboard/states and target detail pages.
//
// Redis structure (docs/gemini/dokumen_arsitektur_aplikasi_web.md:128-130):
//   - target:state:{uuid}  -> JSON status snapshot (model.TargetState)
//   - target:failures:{id} -> consecutive failure counter
//
// Both keys carry the PHP STATE_TTL of 604800 s (7 days).
package probestate

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"opslira/backend/internal/model"
)

const (
	statePrefix   = "target:state:"
	failurePrefix = "target:failures:"

	// StateTTL matches PHP TargetStateService::STATE_TTL = 604800.
	StateTTL = 7 * 24 * time.Hour
)

// Service reads and writes the probe state keys on the DB0 data client.
// A zero-value TargetState is never returned: absent state reads as PENDING,
// like the PHP default array.
type Service struct{ rdb redis.Cmdable }

// New wraps the DB0 (data) Redis client.
func New(rdb redis.Cmdable) *Service { return &Service{rdb: rdb} }

// Get is a package-level convenience for one-off snapshot reads (dashboard
// decoration, target detail). It never errors: an absent or unparseable key
// reads as the PENDING default, matching the PHP TargetStateService::get.
func Get(ctx context.Context, rdb redis.Cmdable, targetID string) (model.TargetState, error) {
	return New(rdb).Get(ctx, targetID), nil
}

func defaultState() model.TargetState {
	return model.TargetState{Status: model.StatusPending, SummaryMetrics: map[string]any{}}
}

// Get returns the stored snapshot, or the PENDING default when the key is
// absent or unparseable.
func (s *Service) Get(ctx context.Context, targetID string) model.TargetState {
	raw, err := s.rdb.Get(ctx, statePrefix+targetID).Result()
	if err != nil {
		return defaultState()
	}
	var state model.TargetState
	if err := json.Unmarshal([]byte(raw), &state); err != nil || state.Status == "" {
		return defaultState()
	}
	if state.SummaryMetrics == nil {
		state.SummaryMetrics = map[string]any{}
	}
	return state
}

// SetStatus stores status + latency + metrics with the 7-day TTL, refreshing
// last_checked_at (PHP setStatus).
func (s *Service) SetStatus(ctx context.Context, targetID, status string, latencyMs *float64, metrics map[string]any) {
	state := s.Get(ctx, targetID)
	state.Status = status
	state.LastLatencyMs = latencyMs
	checkedAt := time.Now().Format(time.RFC3339)
	state.LastCheckedAt = &checkedAt
	if metrics == nil {
		metrics = map[string]any{}
	}
	state.SummaryMetrics = metrics
	s.set(ctx, targetID, state)
}

// IncrementFailures bumps target:failures:{id} (with TTL) and mirrors the
// counter into the state snapshot; returns the new count (PHP incrementFailures).
func (s *Service) IncrementFailures(ctx context.Context, targetID string) int {
	key := failurePrefix + targetID
	count, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0
	}
	s.rdb.Expire(ctx, key, StateTTL)

	state := s.Get(ctx, targetID)
	state.ConsecutiveFailures = int(count)
	s.set(ctx, targetID, state)
	return int(count)
}

// ResetFailures deletes the counter and zeroes it in the snapshot
// (PHP resetFailures).
func (s *Service) ResetFailures(ctx context.Context, targetID string) {
	s.rdb.Del(ctx, failurePrefix+targetID)

	state := s.Get(ctx, targetID)
	state.ConsecutiveFailures = 0
	s.set(ctx, targetID, state)
}

// Failures reads the consecutive failure counter (0 when absent).
func (s *Service) Failures(ctx context.Context, targetID string) int {
	count, err := s.rdb.Get(ctx, failurePrefix+targetID).Int()
	if err != nil {
		return 0
	}
	return count
}

// PreviousStatus is the status before this check; PENDING when unknown.
func (s *Service) PreviousStatus(ctx context.Context, targetID string) string {
	return s.Get(ctx, targetID).Status
}

func (s *Service) set(ctx context.Context, targetID string, state model.TargetState) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return
	}
	s.rdb.Set(ctx, statePrefix+targetID, encoded, StateTTL)
}
