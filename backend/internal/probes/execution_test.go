package probes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/model"
	"opslira/backend/internal/probestate"
)

// fakeStore records the SQL side effects without a database.
type fakeStore struct {
	mu       sync.Mutex
	logs     []ProbeResult
	statuses []string
	resolved int
	opened   int
}

func (f *fakeStore) InsertLog(_ context.Context, _ string, result ProbeResult, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, result)
	return nil
}

func (f *fakeStore) UpdateTargetStatus(_ context.Context, _ model.Target, status string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeStore) TouchTarget(context.Context, model.Target, time.Time) error { return nil }

func (f *fakeStore) OpenIncident(context.Context, model.Target, string, time.Time) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened++
	return "incident-1", nil
}

func (f *fakeStore) ResolveOpenIncidents(context.Context, model.Target, time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved++
	return 1, nil
}

// scriptedAdapter replays a fixed sequence of results, like a target that
// goes DOWN and comes back UP.
type scriptedAdapter struct {
	results []ProbeResult
	calls   int
}

func (a *scriptedAdapter) Configure(map[string]any, map[string]any) Adapter { return a }
func (a *scriptedAdapter) SupportsProtocol(string) bool                     { return true }
func (a *scriptedAdapter) Execute(context.Context) ProbeResult {
	result := a.results[min(a.calls, len(a.results)-1)]
	a.calls++
	return result
}

// staticFactory returns a fixed adapter/error for tests.
type staticFactory struct {
	adapter Adapter
	err     error
}

func (f *staticFactory) Make(string) (Adapter, error) { return f.adapter, f.err }
func (f *staticFactory) SupportedProtocols() []string { return nil }

func newExecutionTest(t *testing.T, adapter Adapter) (*ExecutionService, *fakeStore, *[]string) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := &fakeStore{}
	events := &[]string{}
	return &ExecutionService{
		Factory:   &staticFactory{adapter: adapter},
		State:     probestate.New(client),
		Incidents: store,
		Alert: func(_ context.Context, _ model.Target, event string, _ string) {
			*events = append(*events, event)
		},
	}, store, events
}

// TestExecuteUnknownProtocol surfaces factory errors so the scheduler logs
// them (the 422 mapping belongs to the test-connection handler).
func TestExecuteUnknownProtocol(t *testing.T) {
	service, _, _ := newExecutionTest(t, nil)
	service.Factory = &staticFactory{err: ErrUnknownProtocol}

	_, err := service.Execute(context.Background(), model.Target{
		ID: "t1", ProtocolType: "TELEPATHY", IsActive: true, RetryThreshold: 3,
	}, nil)
	if err == nil {
		t.Fatal("want ErrUnknownProtocol")
	}
}

// TestExecuteSkipsInactive ensures the ExecuteTargetProbeJob re-check: rows
// disabled between claim and run write nothing at all.
func TestExecuteSkipsInactive(t *testing.T) {
	service, store, _ := newExecutionTest(t, &scriptedAdapter{results: []ProbeResult{Up(nil, nil, nil)}})

	_, err := service.Execute(context.Background(), model.Target{
		ID: "t2", ProtocolType: "HTTP_HEALTH", IsActive: false, RetryThreshold: 3,
	}, nil)
	if err != nil {
		t.Fatalf("inactive target must not error: %v", err)
	}
	if len(store.logs) != 0 {
		t.Errorf("inactive target wrote %d logs, want 0", len(store.logs))
	}
}

// TestExecuteDownThenRecover walks the full state machine: DOWN only after
// retry_threshold, incident + alert on the threshold breach, resolution +
// RECOVERED alert on the first UP.
func TestExecuteDownThenRecover(t *testing.T) {
	latency := 12.0
	upLatency := 9.0
	down := Down("Expected HTTP 200, got 500", map[string]any{"http_status": 500}, &latency)
	up := Up(&upLatency, map[string]any{"http_status": 200}, nil)
	service, store, events := newExecutionTest(t,
		&scriptedAdapter{results: []ProbeResult{down, down, up}})

	target := model.Target{
		ID:   "55555555-5555-5555-5555-555555555555",
		Name: "web", TargetType: "WEB_APP", ProtocolType: "HTTP_HEALTH",
		HostOrEndpoint: "https://example.com", IsActive: true, RetryThreshold: 2,
		DegradedThresholdMs: 1000, TimeoutSeconds: 5,
	}
	ctx := context.Background()

	// Failure 1: below threshold, no incident.
	if _, err := service.Execute(ctx, target, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if store.opened != 0 {
		t.Fatalf("incident opened at failure 1 (threshold 2)")
	}
	if got := service.State.Failures(ctx, target.ID); got != 1 {
		t.Fatalf("failures = %d, want 1", got)
	}
	if state := service.State.Get(ctx, target.ID); state.Status != model.StatusPending {
		t.Fatalf("state = %s, want unchanged PENDING below threshold", state.Status)
	}

	// Failure 2: threshold reached -> DOWN state, incident, alert.
	if _, err := service.Execute(ctx, target, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if store.opened != 1 {
		t.Fatalf("incidents opened = %d, want 1 at threshold", store.opened)
	}
	if state := service.State.Get(ctx, target.ID); state.Status != model.StatusDown {
		t.Fatalf("state = %s, want DOWN", state.Status)
	}
	if len(*events) != 1 || (*events)[0] != EventDown {
		t.Fatalf("events = %v, want [DOWN]", *events)
	}

	// The target column flip is recorded exactly once (the threshold breach).
	downFlips := 0
	for _, status := range store.statuses {
		if status == model.StatusDown {
			downFlips++
		}
	}
	if downFlips != 1 {
		t.Fatalf("DOWN column writes = %d, want 1", downFlips)
	}

	// Recovery.
	if _, err := service.Execute(ctx, target, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if store.resolved != 1 {
		t.Fatalf("resolved = %d, want 1", store.resolved)
	}
	if len(*events) != 2 || (*events)[1] != EventRecovered {
		t.Fatalf("events = %v, want [DOWN RECOVERED]", *events)
	}
	if state := service.State.Get(ctx, target.ID); state.Status != model.StatusUp {
		t.Fatalf("state = %s, want UP", state.Status)
	}
	if got := service.State.Failures(ctx, target.ID); got != 0 {
		t.Fatalf("failures after recovery = %d, want 0", got)
	}

	// Every probe wrote a monitoring_logs row.
	if len(store.logs) != 3 {
		t.Fatalf("logs = %d, want 3", len(store.logs))
	}
}

// A failed decrypt must not break the probe: PHP logs a warning and continues
// with empty credentials.
func TestExecuteContinuesAfterDecryptFailure(t *testing.T) {
	service, store, _ := newExecutionTest(t,
		&scriptedAdapter{results: []ProbeResult{Up(nil, nil, nil)}})
	service.Decryptor = failingDecryptor{}

	result, err := service.Execute(context.Background(), model.Target{
		ID: "t3", ProtocolType: "HTTP_HEALTH", IsActive: true, RetryThreshold: 3,
	}, &Credential{EncryptedPayload: "bm90IHZhbGlk", EncryptionIV: "aXY=", EncryptionTag: "dGFn"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != "UP" {
		t.Fatalf("status = %s, want UP despite decrypt failure", result.Status)
	}
	if len(store.logs) != 1 {
		t.Fatalf("logs = %d, want 1", len(store.logs))
	}
}

type failingDecryptor struct{}

func (failingDecryptor) Decrypt(string, string, string) (map[string]any, error) {
	return nil, errors.New("cipher: message authentication failed")
}

// ---- test-connection handler ----

// newTestService builds a Service around a stub factory for handler tests.
func newTestService(t *testing.T, factory Factory) *Service {
	t.Helper()
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &Service{factory: factory, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// The pre-save test route answers the camelCase ProbeResult shape the React
// form reads (TargetFormDrawer.tsx), and unknown protocol_type maps onto the
// Laravel 422 body.
func TestTestConnectionHandlerUnknownProtocol422(t *testing.T) {
	handler := newTestService(t, &staticFactory{err: ErrUnknownProtocol}).TestConnectionHandler()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/targets/test-connection",
		strings.NewReader(`{"name":"X","target_type":"WEB_APP","protocol_type":"CARRIER_PIGEON","host_or_endpoint":"https://example.com"}`))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["message"] != "unknown protocol_type" {
		t.Errorf("message = %v, want %q", body["message"], "unknown protocol_type")
	}
}

// The handler configures the adapter from the raw form payload and returns
// its result without persisting anything.
func TestTestConnectionHandlerPassesFormToAdapter(t *testing.T) {
	recording := &recordingAdapter{result: Up(nil, map[string]any{"http_status": 200}, nil)}
	handler := newTestService(t, &recordingFactory{adapter: recording}).TestConnectionHandler()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/targets/test-connection",
		strings.NewReader(`{
			"name":"Example",
			"dashboard_group":"APP_DOMAIN_DB",
			"target_type":"WEB_APP",
			"protocol_type":"HTTP_HEALTH",
			"host_or_endpoint":"https://example.com",
			"port":443,
			"timeout_seconds":7,
			"config_payload":{"method":"HEAD"},
			"credentials":{"auth_type":"NONE"}
		}`))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if recording.timeoutSeconds != 7 {
		t.Errorf("timeout = %d, want the form's 7", recording.timeoutSeconds)
	}
	if recording.protocolType != ProtocolHTTPHealth {
		t.Errorf("protocol = %s, want HTTP_HEALTH", recording.protocolType)
	}
	if recording.config["host"] != "https://example.com" || recording.config["method"] != "HEAD" {
		t.Errorf("config = %v, want host+method injected", recording.config)
	}
	if recording.credentials != nil {
		t.Errorf("credentials = %v, want nil for auth_type NONE", recording.credentials)
	}

	// The form reads these exact camelCase keys.
	var raw map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &raw)
	for _, key := range []string{"status", "responseTimeMs", "errorMessage", "metricsPayload"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response missing camelCase key %q", key)
		}
	}
}

// An INTERNET target is ping-only regardless of what the client claims: the
// protocol is forced to ICMP_PING server-side (normalizeTargetPayload) and
// icmp_only lands in the config.
func TestTestConnectionHandlerForcesInternetProtocol(t *testing.T) {
	recording := &recordingAdapter{result: Down("Ping failed (host not found): no output from ping", nil, nil)}
	handler := newTestService(t, &recordingFactory{adapter: recording}).TestConnectionHandler()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/targets/test-connection",
		strings.NewReader(`{
			"name":"Home line",
			"target_type":"INTERNET",
			"protocol_type":"HTTP_HEALTH",
			"host_or_endpoint":"192.0.2.1",
			"timeout_seconds":1
		}`))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if recording.config["icmp_only"] != true {
		t.Errorf("icmp_only = %v, want true", recording.config["icmp_only"])
	}
	if recording.protocolType != ProtocolICMPPing {
		t.Errorf("protocol = %s, want ICMP_PING forced", recording.protocolType)
	}
}

// recordingAdapter captures what the handler configured.
type recordingAdapter struct {
	protocolType   string
	config         map[string]any
	credentials    map[string]any
	timeoutSeconds int
	result         ProbeResult
}

func (a *recordingAdapter) Configure(config, credentials map[string]any) Adapter {
	a.config = config
	a.credentials = credentials
	return a
}
func (a *recordingAdapter) Tune(timeoutSeconds, _ int) Adapter {
	a.timeoutSeconds = timeoutSeconds
	return a
}
func (a *recordingAdapter) SupportsProtocol(string) bool { return true }
func (a *recordingAdapter) Execute(context.Context) ProbeResult {
	return a.result
}

// recordingFactory records the normalized protocol the handler resolved.
type recordingFactory struct{ adapter *recordingAdapter }

func (f *recordingFactory) Make(protocolType string) (Adapter, error) {
	f.adapter.protocolType = protocolType
	return f.adapter, nil
}
func (f *recordingFactory) SupportedProtocols() []string { return nil }

func TestMaskHidesCredentials(t *testing.T) {
	cases := map[string]string{
		"Authorization: Bearer sekret": "Authorization: ****",
		"password=hunter2 remains":     "password=**** remains",
		"dsn postgres://user:pw@db/x":  "dsn postgres://user:****@db/x",
		"community: public-snmp":       "community: ****",
	}
	for input, want := range cases {
		if got := Mask(input); !strings.Contains(got, want) {
			t.Errorf("Mask(%q) = %q, want it to contain %q", input, got, want)
		}
	}
}
