package probes

import (
	"context"
	"log/slog"
	"regexp"

	"opslira/backend/internal/model"
)

// One warning line for every non-UP target check, so the log names the
// target, the phase, and the reason instead of only the happy path. Monitoring
// rows carry history; this is the diagnostic trail, and every string is masked
// before it is written so a credential never lands in a log file
// (ProbeFailureLogger).

const maskMaxLength = 1000 // Longest message kept; adapter errors can carry an HTTP body.

var (
	// "Authorization: Bearer x" / "Authorization: Basic x".
	maskAuthorization = regexp.MustCompile(`(?i)(Authorization:\s*)(?:Bearer\s+|Basic\s+)?\S+`)
	// password=..., token: ..., community = ... — the label stays, the value goes.
	maskKeyValue = regexp.MustCompile(`(?i)((?:password|passwd|pwd|secret(?:_key)?|api[_-]?key|access[_-]?key|bearer_token|token|community|identity|auth_password|priv_password|username|user)\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s@]+)`)
	// scheme://user:pass@host credentials inside URLs/DSNs.
	maskURLCredentials = regexp.MustCompile(`(://[^/\s:@]+:)[^/\s@]+@`)
)

// Mask replaces credential-shaped values with ****. Applied to the whole
// message, not to a known credential list: an error string is built by the
// adapter and can quote a request body, a DSN, or a command line.
func Mask(text string) string {
	masked := maskAuthorization.ReplaceAllString(text, "${1}****")
	masked = maskKeyValue.ReplaceAllString(masked, "${1}****")
	masked = maskURLCredentials.ReplaceAllString(masked, "${1}****@")

	runes := []rune(masked)
	if len(runes) > maskMaxLength {
		masked = string(runes[:maskMaxLength])
	}
	return masked
}

// MaskValue masks every string inside a metrics map, recursively.
func MaskValue(value any) any {
	switch v := value.(type) {
	case string:
		return Mask(v)
	case map[string]any:
		masked := make(map[string]any, len(v))
		for key, item := range v {
			masked[key] = MaskValue(item)
		}
		return masked
	case []any:
		masked := make([]any, len(v))
		for i, item := range v {
			masked[i] = MaskValue(item)
		}
		return masked
	default:
		return value
	}
}

// LogFailure writes the one warning line for a non-UP result (phase is
// "scheduled" or "test"); UP results are silent.
func LogFailure(ctx context.Context, logger *slog.Logger, t model.Target, result ProbeResult, phase string) {
	if result.Status == "UP" {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	errMessage := any(nil)
	if result.ErrorMessage != nil {
		errMessage = Mask(*result.ErrorMessage)
	}
	latency := any(nil)
	if result.ResponseTimeMs != nil {
		latency = *result.ResponseTimeMs
	}

	logger.WarnContext(ctx, "Target probe failed",
		"target_id", t.ID,
		"target_name", Mask(t.Name),
		"target_type", t.TargetType,
		"protocol_type", t.ProtocolType,
		"host", Mask(t.HostOrEndpoint),
		"phase", phase,
		"status", result.Status,
		"error_message", errMessage,
		"response_time_ms", latency,
		"metrics_payload", MaskValue(result.MetricsPayload),
	)
}
