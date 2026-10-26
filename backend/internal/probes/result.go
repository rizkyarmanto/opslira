// Package probes ports the PHP probe pipeline (app/Modules/Probes): a
// protocol_type resolves to an Adapter through the Factory, the adapter is
// configured with the target's config_payload plus decrypted credentials and
// executed, and the ProbeResult is written to monitoring_logs + Redis state by
// the ExecutionService.
//
// There is no equivalent of inertia/predis/Eloquent here: adapters talk to the
// network directly, persistence is raw SQL through pgx, state is Redis.
package probes

// ProbeResult mirrors App\Modules\Probes\Contracts\ProbeResult. The JSON keys
// are the camelCase ones ProbeResult::toArray() emits, because the React form
// (TargetFormDrawer.tsx) reads them straight off POST /targets/test-connection.
type ProbeResult struct {
	Status         string         `json:"status"` // UP | DEGRADED | DOWN
	ResponseTimeMs *float64       `json:"responseTimeMs"`
	ErrorMessage   *string        `json:"errorMessage"`
	MetricsPayload map[string]any `json:"metricsPayload"`
	HTTPStatusCode *int           `json:"httpStatusCode"`
}

// Up builds an UP result.
func Up(latencyMs *float64, metrics map[string]any, httpStatus *int) ProbeResult {
	return ProbeResult{Status: "UP", ResponseTimeMs: latencyMs, MetricsPayload: metrics, HTTPStatusCode: httpStatus}
}

// Degraded builds a DEGRADED result with an operator-facing reason.
func Degraded(latencyMs *float64, metrics map[string]any, reason string) ProbeResult {
	return ProbeResult{Status: "DEGRADED", ResponseTimeMs: latencyMs, ErrorMessage: &reason, MetricsPayload: metrics}
}

// Down builds a DOWN result. An empty metrics map is normalised to an empty
// (non-nil) object so JSON serialises as {} like PHP's [] array cast.
func Down(message string, metrics map[string]any, latencyMs *float64) ProbeResult {
	if metrics == nil {
		metrics = map[string]any{}
	}
	return ProbeResult{Status: "DOWN", ResponseTimeMs: latencyMs, ErrorMessage: &message, MetricsPayload: metrics}
}

// IsUp reports whether the probe status is UP (recovery checks use this, not
// "not DOWN": DEGRADED is not a recovery).
func (r ProbeResult) IsUp() bool { return r.Status == "UP" }

// IsDown reports whether the probe status is DOWN (failure counting uses this;
// DEGRADED does not count as a failure).
func (r ProbeResult) IsDown() bool { return r.Status == "DOWN" }
