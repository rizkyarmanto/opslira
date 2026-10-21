// Package client talks to the Laravel internal runner endpoints.
//
// Every request carries the runner token in X-Opslira-Runner and a per-call
// timeout sized to the slowest thing that endpoint can do: a claim is quick, a
// probe runs an adapter with its own retries, and a report render can take
// minutes.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"opslira/runner/internal/config"
)

const (
	claimTimeout      = 30 * time.Second
	probeTimeout      = 45 * time.Second
	reportTimeout     = 240 * time.Second
	maintenanceTimeout = 300 * time.Second

	maxErrorBody = 500
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type Target struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProbeOutcome struct {
	Status          string   `json:"status"`
	ResponseTimeMs  *float64 `json:"response_time_ms"`
	ErrorMessage    *string  `json:"error_message"`
}

func New(cfg config.Config) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		// Timeouts are set per call, so the transport client itself stays open.
		http: &http.Client{},
	}
}

// Claim returns the targets whose next_check_at has passed.
func (c *Client) Claim(ctx context.Context, limit int) ([]Target, error) {
	var response struct {
		Targets []Target `json:"targets"`
	}
	if err := c.post(ctx, "/internal/probes/claim", map[string]int{"limit": limit}, claimTimeout, &response); err != nil {
		return nil, err
	}

	return response.Targets, nil
}

// RunProbe executes one probe through the PHP adapter pipeline.
func (c *Client) RunProbe(ctx context.Context, id string) (ProbeOutcome, error) {
	var outcome ProbeOutcome
	if err := c.post(ctx, "/internal/probes/"+id, nil, probeTimeout, &outcome); err != nil {
		return ProbeOutcome{}, err
	}

	return outcome, nil
}

// DispatchReports generates every report schedule that is due right now.
func (c *Client) DispatchReports(ctx context.Context) (int, error) {
	var response struct {
		Dispatched int `json:"dispatched"`
	}
	if err := c.post(ctx, "/internal/reports/dispatch", nil, reportTimeout, &response); err != nil {
		return 0, err
	}

	return response.Dispatched, nil
}

// RunMaintenance runs a named Artisan maintenance task ("rollup" or "prune").
func (c *Client) RunMaintenance(ctx context.Context, task string) (string, error) {
	var response struct {
		OK     bool   `json:"ok"`
		Output string `json:"output"`
	}
	if err := c.post(ctx, "/internal/maintenance/"+task, nil, maintenanceTimeout, &response); err != nil {
		return "", err
	}

	return response.Output, nil
}

func (c *Client) post(ctx context.Context, path string, body any, timeout time.Duration, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("runner api %s: encode: %w", path, err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("runner api %s: %w", path, err)
	}
	req.Header.Set("X-Opslira-Runner", c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("runner api %s: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("runner api %s: read: %w", path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("runner api %s: %s", path, truncate(string(data), maxErrorBody))
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("runner api %s: decode: %w", path, err)
		}
	}

	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return value[:limit]
}
