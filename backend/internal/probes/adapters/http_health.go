package adapters

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"opslira/backend/internal/probes"
)

// maxBodyRead bounds how much of a response body the JSON assertion reads;
// the PHP client read it all, but an unbounded read invites a huge page to
// stall the probe pool.
const maxBodyRead = 2 << 20 // 2 MiB

// HTTPHealth probes WEB_APP targets: HTTP/HTTPS health check + optional JSON
// key/value assertion (HttpHealthProbeAdapter).
type HTTPHealth struct{ adapterBase }

// NewHTTPHealth builds the HTTP_HEALTH adapter.
func NewHTTPHealth() *HTTPHealth { return &HTTPHealth{adapterBase: newBase()} }

func (a *HTTPHealth) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolHTTPHealth
}

func (a *HTTPHealth) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *HTTPHealth) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}

func (a *HTTPHealth) Execute(ctx context.Context) probes.ProbeResult {
	// WEB_APP targets carry the full URL in host_or_endpoint (injected as
	// config "host"); config_payload "url" stays for hand-built payloads.
	url := a.cfgString("url", "")
	if url == "" {
		url = a.cfgString("host", "")
	}
	if url == "" {
		return probes.Down("Missing url in config_payload", nil, nil)
	}

	method := strings.ToUpper(a.cfgString("method", "GET"))
	expectedStatus := a.cfgInt("expected_status", 200)
	verifySSL := a.cfgBool("verify_ssl", true)

	request, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return probes.Down("HTTP probe error: "+err.Error(), nil, nil)
	}
	request.Header.Set("User-Agent", "Opslira-Monitor/1.0")

	if token := a.credString("bearer_token"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	} else if username := a.credString("username"); username != "" {
		request.SetBasicAuth(username, a.credString("password"))
	}

	client := &http.Client{
		Timeout: time.Duration(a.timeoutSeconds) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: !verifySSL}, //nolint:gosec // verify_ssl=false is an explicit target option, as in PHP
			ForceAttemptHTTP2: true,
		},
	}

	start := time.Now()
	response, err := client.Do(request)
	latency := round2(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		return probes.Down("HTTP probe error: "+err.Error(), nil, nil)
	}
	defer response.Body.Close()

	metrics := map[string]any{
		"http_status":  response.StatusCode,
		"content_type": response.Header.Get("Content-Type"),
	}

	if response.StatusCode != expectedStatus {
		return probes.Down(
			fmt.Sprintf("Expected HTTP %d, got %d", expectedStatus, response.StatusCode),
			metrics,
			msPointer(latency),
		)
	}

	// Optional JSON key/value assertion (data_get dot notation). The body is
	// read once, bounded, so a huge page cannot stall the probe pool.
	jsonKey := a.cfgString("expected_json_key", "")
	var body []byte
	if jsonKey != "" || response.StatusCode/100 == 2 {
		body, _ = io.ReadAll(io.LimitReader(response.Body, maxBodyRead))
	}
	if jsonKey != "" {
		var parsed map[string]any
		// An undecodable body leaves actual nil, exactly like PHP's
		// $response->json() returning null through data_get.
		_ = json.Unmarshal(body, &parsed)
		actual := dataGet(parsed, jsonKey)
		expectedValue, hasExpected := a.config["expected_json_value"]

		metrics["json_key"] = jsonKey
		metrics["json_value"] = actual

		if hasExpected && expectedValue != nil && toString(actual) != toString(expectedValue) {
			return probes.Down(
				fmt.Sprintf("JSON key '%s' = '%v', expected '%v'", jsonKey, actual, expectedValue),
				metrics,
				msPointer(latency),
			)
		}
	}

	status := a.classifyLatency(msPointer(latency))
	httpStatus := response.StatusCode
	return probes.ProbeResult{
		Status:         status,
		ResponseTimeMs: msPointer(latency),
		MetricsPayload: metrics,
		HTTPStatusCode: &httpStatus,
	}
}
