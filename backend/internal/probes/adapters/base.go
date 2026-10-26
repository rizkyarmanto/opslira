// Package adapters holds the eight probe adapters, one per protocol_type
// family, mirroring app/Modules/Probes/Adapters. Every adapter embeds
// adapterBase, which carries the config/credential accessors and the latency
// classification of BaseProbeAdapter.
package adapters

import (
	"math"
	"strconv"
	"strings"

	"opslira/backend/internal/model"
)

type adapterBase struct {
	config              map[string]any
	credentials         map[string]any
	timeoutSeconds      int
	degradedThresholdMs int
}

func newBase() adapterBase {
	return adapterBase{config: map[string]any{}, credentials: map[string]any{}, timeoutSeconds: 5, degradedThresholdMs: 1000}
}

// apply is base Configure(): PHP stores the maps by reference and adapters
// never mutate them.
func (b *adapterBase) apply(config, credentials map[string]any) {
	if config == nil {
		config = map[string]any{}
	}
	if credentials == nil {
		credentials = map[string]any{}
	}
	b.config = config
	b.credentials = credentials
}

// tune is forTarget()/withTimeoutSeconds(): values <= 0 keep the PHP defaults
// (timeout 5 s, degraded threshold 1000 ms).
func (b *adapterBase) tune(timeoutSeconds, degradedThresholdMs int) {
	if timeoutSeconds > 0 {
		b.timeoutSeconds = max(1, min(60, timeoutSeconds))
	}
	if degradedThresholdMs > 0 {
		b.degradedThresholdMs = degradedThresholdMs
	}
}

// classifyLatency: latency above the target's degraded_threshold_ms is
// DEGRADED; a nil latency stays UP (PHP classifyLatency).
func (b *adapterBase) classifyLatency(latencyMs *float64) string {
	if latencyMs == nil {
		return model.StatusUp
	}
	if *latencyMs > float64(b.degradedThresholdMs) {
		return model.StatusDegraded
	}
	return model.StatusUp
}

func (b *adapterBase) cfgString(key, fallback string) string {
	v, ok := b.config[key]
	if !ok || v == nil {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return toString(v)
}

func (b *adapterBase) cfgInt(key string, fallback int) int {
	v, ok := b.config[key]
	if !ok || v == nil {
		return fallback
	}
	return toInt(v, fallback)
}

// cfgBool implements filter_var(..., FILTER_VALIDATE_BOOLEAN): "true","1",
// "yes","on" are true; the literal string "false" is NOT truthy, which is
// exactly why PHP wraps every form checkbox in filter_var.
func (b *adapterBase) cfgBool(key string, fallback bool) bool {
	v, ok := b.config[key]
	if !ok || v == nil {
		return fallback
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off", "":
			return false
		default:
			return fallback
		}
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return fallback
	}
}

// credString returns a credential as a string; missing/non-string values map
// to "" so the PHP guard is_string($x) && $x !== ” translates directly.
func (b *adapterBase) credString(key string) string {
	v, ok := b.credentials[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return toString(v)
}

// credNonEmpty mirrors the multi-key credential presence checks (e.g. SMTP
// AUTH needs both username and password).
func (b *adapterBase) credNonEmpty(keys ...string) bool {
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if b.credString(key) == "" {
			return false
		}
	}
	return true
}

// round2 mirrors PHP round($x, 2) for latency milliseconds.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func msPointer(v float64) *float64 { return &v }

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		// PHP string casts: (string) true === "1", (string) false === "".
		if t {
			return "1"
		}
		return ""
	default:
		return ""
	}
}

func toInt(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case *int:
		if t == nil {
			return fallback
		}
		return *t
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return int(f)
		}
		return fallback
	default:
		return fallback
	}
}

func toFloat(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f
		}
		return fallback
	default:
		return fallback
	}
}

// dataGet implements Laravel's data_get($body, 'a.b.c') for JSON documents.
func dataGet(v any, dotPath string) any {
	current := v
	for _, part := range strings.Split(dotPath, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// limitString mirrors Str::limit($s, 300): bounded body excerpts in error
// messages, with a trailing "..." when truncated.
func limitString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
