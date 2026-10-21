package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"opslira/runner/internal/client"
	"opslira/runner/internal/config"
)

// The shutdown contract: a signal stops claiming, the in-flight probe is
// allowed to finish rather than being cancelled, and Run returns nil.
func TestRunShutsDownCleanlyAndLetsInFlightProbeFinish(t *testing.T) {
	var claims atomic.Int64
	var probeStarted atomic.Bool
	probeDone := make(chan struct{})
	var probeDoneOnce sync.Once

	mux := http.NewServeMux()
	mux.HandleFunc("/internal/probes/claim", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Hand out one target on the first claim only, so the test drives a
		// single in-flight probe.
		if claims.Add(1) == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"targets": []map[string]string{{"id": "t1", "name": "Target One"}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"targets": []map[string]string{}})
	})
	mux.HandleFunc("/internal/probes/t1", func(w http.ResponseWriter, r *http.Request) {
		probeStarted.Store(true)
		time.Sleep(2 * time.Second)
		probeDoneOnce.Do(func() { close(probeDone) })
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "UP"})
	})
	mux.HandleFunc("/internal/reports/dispatch", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"dispatched": 0})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := config.Config{
		BaseURL:           server.URL,
		Token:             "test-token",
		ProbeConcurrency:  2,
		TickSeconds:       1,
		ReportTickSeconds: 3600,
		RollupAt:          "00:05",
		PruneAt:           "02:00",
		ListenAddr:        "127.0.0.1:0",
		Timezone:          "Asia/Jakarta",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- New(cfg, client.New(cfg)).Run(ctx) }()

	deadline := time.Now().Add(10 * time.Second)
	for !probeStarted.Load() {
		if time.Now().After(deadline) {
			t.Fatal("no probe reached the server")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned an error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return within the shutdown grace")
	}

	select {
	case <-probeDone:
	default:
		t.Fatal("the in-flight probe was abandoned instead of being allowed to finish")
	}
}
