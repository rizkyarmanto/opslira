package reporting

import (
	"database/sql"
	"encoding/json"
	"time"
)

// strPtr/timePtrV convert nullable scan results into DTO pointers.
func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}

	return &n.String
}

func timePtrV(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}

	return &n.Time
}

// decodeObject turns a jsonb column into a map; empty yields an object so
// scope_filter/summary_snapshot never serialize as null.
func decodeObject(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}

	return out
}

// decodeStringArray turns a jsonb string array column into a slice; empty
// yields [] (Laravel casts).
func decodeStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []string{}
	}

	return out
}

// anyToStrings recovers a []string from a decoded scope_filter value.
func anyToStrings(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}

		return out, true
	default:
		return nil, false
	}
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}

// timezoneOr resolves tz, falling back to the configured scheduler zone.
func timezoneOr(tz string, fallback *time.Location) *time.Location {
	if tz == "" {
		return fallback
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}

	return fallback
}
