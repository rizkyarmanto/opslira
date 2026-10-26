package probestate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/model"
)

func newService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), mini
}

// The PHP TargetStateService writes target:state:{uuid} with a 7-day TTL; the
// dashboard and target detail read that exact JSON shape.
func TestSetStatusWritesStateWithSevenDayTTL(t *testing.T) {
	service, mini := newService(t)
	ctx := context.Background()

	latency := 42.5
	service.SetStatus(ctx, "11111111-1111-1111-1111-111111111111", model.StatusUp, &latency,
		map[string]any{"http_status": 200})

	raw, err := mini.Get("target:state:11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("state key missing: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("state is not JSON: %v", err)
	}
	if stored["status"] != "UP" {
		t.Errorf("status = %v, want UP", stored["status"])
	}
	if stored["last_latency_ms"] != 42.5 {
		t.Errorf("last_latency_ms = %v, want 42.5", stored["last_latency_ms"])
	}
	if stored["last_checked_at"] == nil || stored["last_checked_at"] == "" {
		t.Error("last_checked_at missing")
	}
	metrics, ok := stored["summary_metrics"].(map[string]any)
	if !ok || metrics["http_status"] != float64(200) {
		t.Errorf("summary_metrics = %v, want http_status 200", stored["summary_metrics"])
	}

	ttl := mini.TTL("target:state:11111111-1111-1111-1111-111111111111")
	if ttl != StateTTL {
		t.Errorf("state TTL = %v, want %v (604800 s)", ttl, StateTTL)
	}
	if StateTTL != 604800*time.Second {
		t.Errorf("StateTTL = %v, want 604800s like PHP STATE_TTL", StateTTL)
	}
}

func TestFailuresCounterTTLAndReset(t *testing.T) {
	service, mini := newService(t)
	ctx := context.Background()
	targetID := "22222222-2222-2222-2222-222222222222"

	if got := service.IncrementFailures(ctx, targetID); got != 1 {
		t.Fatalf("first increment = %d, want 1", got)
	}
	if got := service.IncrementFailures(ctx, targetID); got != 2 {
		t.Fatalf("second increment = %d, want 2", got)
	}
	if ttl := mini.TTL("target:failures:" + targetID); ttl != StateTTL {
		t.Errorf("failures TTL = %v, want %v", ttl, StateTTL)
	}
	if got := service.Failures(ctx, targetID); got != 2 {
		t.Errorf("Failures() = %d, want 2", got)
	}
	if got := service.PreviousStatus(ctx, targetID); got != model.StatusPending {
		t.Errorf("PreviousStatus() = %s, want PENDING before any SetStatus", got)
	}

	service.ResetFailures(ctx, targetID)
	if got := service.Failures(ctx, targetID); got != 0 {
		t.Errorf("Failures() after reset = %d, want 0", got)
	}
	if mini.Exists("target:failures:" + targetID) {
		t.Error("failures key survived reset")
	}
	state := service.Get(ctx, targetID)
	if state.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, want 0", state.ConsecutiveFailures)
	}
	if state.Status != model.StatusPending {
		t.Errorf("status after reset = %s, want unchanged PENDING", state.Status)
	}
}

func TestGetDefaultsToPending(t *testing.T) {
	service, _ := newService(t)

	state := service.Get(context.Background(), "33333333-3333-3333-3333-333333333333")
	if state.Status != model.StatusPending {
		t.Errorf("status = %s, want PENDING", state.Status)
	}
	if state.SummaryMetrics == nil {
		t.Error("summary_metrics must be an empty map, not nil (JSON {} like PHP)")
	}
}

// The failure counter and the snapshot must both survive 7 days, matching the
// PHP STATE_TTL on both keys.
func TestBothKeysCarrySevenDayTTLAfterDown(t *testing.T) {
	service, mini := newService(t)
	ctx := context.Background()
	targetID := "44444444-4444-4444-4444-444444444444"

	service.IncrementFailures(ctx, targetID)

	state := service.Get(ctx, targetID)
	if state.Status != model.StatusPending {
		t.Fatalf("increment must not flip status, got %s", state.Status)
	}
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("consecutive_failures = %d, want 1 mirrored into snapshot", state.ConsecutiveFailures)
	}
	if ttl := mini.TTL("target:state:" + targetID); ttl != StateTTL {
		t.Errorf("state TTL after increment = %v, want %v", ttl, StateTTL)
	}
}
