package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHealthUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "Opslira-Monitor/1.0" {
			t.Errorf("User-Agent = %q, want Opslira-Monitor/1.0", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	adapter := NewHTTPHealth().Tune(5, 1000).Configure(
		map[string]any{"host": server.URL, "expected_status": 200},
		nil,
	)
	result := adapter.Execute(context.Background())

	if result.Status != "UP" {
		t.Fatalf("status = %s, want UP (error: %v)", result.Status, result.ErrorMessage)
	}
	if result.HTTPStatusCode == nil || *result.HTTPStatusCode != 200 {
		t.Errorf("http status = %v, want 200", result.HTTPStatusCode)
	}
	if result.ResponseTimeMs == nil {
		t.Error("response_time_ms is nil")
	}
	if result.MetricsPayload["http_status"] != 200 {
		t.Errorf("metrics http_status = %v, want 200", result.MetricsPayload["http_status"])
	}
}

func TestHTTPHealthDownOnStatusMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	adapter := NewHTTPHealth().Tune(5, 1000).Configure(
		map[string]any{"host": server.URL, "expected_status": 200},
		nil,
	)
	result := adapter.Execute(context.Background())

	if result.Status != "DOWN" {
		t.Fatalf("status = %s, want DOWN", result.Status)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage != "Expected HTTP 200, got 500" {
		t.Errorf("error = %v, want %q", result.ErrorMessage, "Expected HTTP 200, got 500")
	}
	if result.MetricsPayload["http_status"] != 500 {
		t.Errorf("metrics http_status = %v, want 500", result.MetricsPayload["http_status"])
	}
}

func TestHTTPHealthDownOnConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close() // nothing listens any more

	adapter := NewHTTPHealth().Tune(2, 1000).Configure(map[string]any{"host": url}, nil)
	result := adapter.Execute(context.Background())

	if result.Status != "DOWN" {
		t.Fatalf("status = %s, want DOWN", result.Status)
	}
	if result.ErrorMessage == nil || !strings.HasPrefix(*result.ErrorMessage, "HTTP probe error: ") {
		t.Errorf("error = %v, want HTTP probe error prefix", result.ErrorMessage)
	}
}

func TestHTTPHealthBearerAndJSONAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sekret" {
			t.Errorf("Authorization = %q, want Bearer sekret", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "2.1.0"}})
	}))
	defer server.Close()

	adapter := NewHTTPHealth().Tune(5, 1000).Configure(
		map[string]any{"host": server.URL, "expected_json_key": "data.version", "expected_json_value": "2.1.0"},
		map[string]any{"bearer_token": "sekret"},
	)
	if result := adapter.Execute(context.Background()); result.Status != "UP" {
		t.Fatalf("status = %s, want UP (error: %v)", result.Status, result.ErrorMessage)
	}

	adapter = NewHTTPHealth().Tune(5, 1000).Configure(
		map[string]any{"host": server.URL, "expected_json_key": "data.version", "expected_json_value": "3.0.0"},
		map[string]any{"bearer_token": "sekret"},
	)
	result := adapter.Execute(context.Background())
	if result.Status != "DOWN" {
		t.Fatalf("status = %s, want DOWN on JSON mismatch", result.Status)
	}
	if result.ErrorMessage == nil || !strings.Contains(*result.ErrorMessage, "data.version") {
		t.Errorf("error = %v, want data.version mention", result.ErrorMessage)
	}
}
