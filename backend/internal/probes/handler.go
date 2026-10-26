package probes

import (
	"encoding/json"
	"errors"
	"net/http"

	"opslira/backend/internal/model"
)

// testConnectionRequest mirrors the raw form payload TargetFormDrawer.tsx
// posts to POST /api/v1/targets/test-connection (plaintext credentials: the
// target does not exist yet).
type testConnectionRequest struct {
	Name           string         `json:"name"`
	DashboardGroup string         `json:"dashboard_group"`
	TargetType     string         `json:"target_type"`
	ProtocolType   string         `json:"protocol_type"`
	HostOrEndpoint string         `json:"host_or_endpoint"`
	Port           *int           `json:"port"`
	ConfigPayload  map[string]any `json:"config_payload"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Credentials    *struct {
		AuthType string         `json:"auth_type"`
		Payload  map[string]any `json:"payload"`
	} `json:"credentials"`
}

// testConnectionHandler is the pre-save connection test (the Service exposes
// it through TestConnectionHandler): it runs the adapter without persisting a
// monitoring_logs row, maps ErrUnknownProtocol onto the Laravel 422 body
// {"message":"unknown protocol_type"}, and always answers the camelCase
// ProbeResult shape (status/responseTimeMs/errorMessage/metricsPayload),
// exactly like TargetController::testConnectionPayload.
func testConnectionHandler(s *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request testConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "The given data was invalid."})
			return
		}

		protocolType := normalizeProtocol(request.TargetType, request.ProtocolType)
		adapter, err := s.factory.Make(protocolType)
		if err != nil {
			if errors.Is(err, ErrUnknownProtocol) {
				// Exact Laravel-shaped message the frontend contract expects.
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "unknown protocol_type"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": err.Error()})
			return
		}

		// Same config assembly as the saved-target path, plus the payload
		// normalisations of normalizeTargetPayload.
		config := make(map[string]any, len(request.ConfigPayload)+2)
		for key, value := range request.ConfigPayload {
			config[key] = value
		}
		if request.TargetType == "INTERNET" {
			// INTERNET targets are ping only; the protocol is forced above.
			config["icmp_only"] = true
		} else {
			delete(config, "icmp_only")
		}
		if request.TargetType == "BESZEL_SERVER" {
			// Rows saved by the previous form keep working.
			if _, ok := config["beszel_system_name"]; !ok {
				if id, ok := config["beszel_system_id"]; ok {
					config["beszel_system_name"] = id
				}
			}
			delete(config, "beszel_system_id")
		}
		config["host"] = request.HostOrEndpoint
		config["port"] = request.Port

		var credentials map[string]any
		if request.Credentials != nil && request.Credentials.AuthType != "" && request.Credentials.AuthType != "NONE" {
			credentials = request.Credentials.Payload
		}

		if tuned, ok := adapter.(TunedAdapter); ok {
			// The pre-save test has no target row, so the form timeout must be
			// applied explicitly instead of the 5 s default (PHP withTimeoutSeconds).
			adapter = tuned.Tune(request.TimeoutSeconds, 1000)
		}
		adapter.Configure(config, credentials)

		result := adapter.Execute(r.Context())

		// The failure logger still needs name, type and host to say which
		// check failed, so it gets a transient target with the form values.
		transient := model.Target{
			Name:           request.Name,
			DashboardGroup: request.DashboardGroup,
			TargetType:     request.TargetType,
			ProtocolType:   protocolType,
			HostOrEndpoint: request.HostOrEndpoint,
		}
		if transient.Name == "" {
			transient.Name = "Unsaved target test"
		}
		LogFailure(r.Context(), s.logger, transient, result, "test")

		writeJSON(w, http.StatusOK, result)
	}
}

// normalizeProtocol mirrors normalizeTargetPayload: INTERNET targets are ping
// only, so the protocol is forced server-side — a client cannot switch the
// target back onto TCP fallback or SNMP (InternetTargetTest).
func normalizeProtocol(targetType, protocolType string) string {
	if targetType == "INTERNET" {
		return ProtocolICMPPing
	}
	return protocolType
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
