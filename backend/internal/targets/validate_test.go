package targets

import (
	"strings"
	"testing"
)

func basePayload() TargetPayload {
	port := 443

	return TargetPayload{
		Name:           "Portal",
		DashboardGroup: "APP_DOMAIN_DB",
		TargetType:     "WEB_APP",
		Environment:    "PRODUCTION",
		HostOrEndpoint: "https://portal.example.id/health",
		Port:           &port,
		ProtocolType:   "HTTP_HEALTH",
		ConfigPayload:  map[string]any{},
	}
}

func TestValidateAcceptsValidPayload(t *testing.T) {
	if errs := validate(basePayload()); len(errs) > 0 {
		t.Errorf("validate() = %v, want no errors", errs)
	}
}

func TestValidateRejectsBadEnumsAndRanges(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*TargetPayload)
		key    string
	}{
		{"bad dashboard group", func(p *TargetPayload) { p.DashboardGroup = "OTHER" }, "dashboard_group"},
		{"bad target type", func(p *TargetPayload) { p.TargetType = "PRINTER" }, "target_type"},
		{"bad environment", func(p *TargetPayload) { p.Environment = "UAT" }, "environment"},
		{"bad protocol", func(p *TargetPayload) { p.ProtocolType = "TELNET" }, "protocol_type"},
		{"missing name", func(p *TargetPayload) { p.Name = "" }, "name"},
		{"missing host", func(p *TargetPayload) { p.HostOrEndpoint = "" }, "host_or_endpoint"},
		{"port zero", func(p *TargetPayload) { p.Port = ptrTo(0) }, "port"},
		{"port too high", func(p *TargetPayload) { p.Port = ptrTo(70000) }, "port"},
		{"interval too small", func(p *TargetPayload) { p.CheckIntervalSeconds = ptrTo(5) }, "check_interval_seconds"},
		{"timeout out of range", func(p *TargetPayload) { p.TimeoutSeconds = ptrTo(61) }, "timeout_seconds"},
		{"retry out of range", func(p *TargetPayload) { p.RetryThreshold = ptrTo(11) }, "retry_threshold"},
		{"degraded zero", func(p *TargetPayload) { p.DegradedThresholdMs = ptrTo(0) }, "degraded_threshold_ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := basePayload()
			tc.mutate(&payload)
			errs := validate(payload)
			if _, ok := errs[tc.key]; !ok {
				t.Errorf("expected error for %q, got %v", tc.key, errs)
			}
		})
	}
}

func TestValidateInternetTypeAllowsPingProtocol(t *testing.T) {
	payload := basePayload()
	payload.TargetType = "INTERNET"
	payload.ProtocolType = "ICMP_PING"
	if errs := validate(payload); len(errs) > 0 {
		t.Errorf("INTERNET + ICMP_PING rejected: %v", errs)
	}
}

func TestValidateCredentialMatrix(t *testing.T) {
	t.Run("SNMP_V2C needs community", func(t *testing.T) {
		payload := basePayload()
		payload.Credentials = &CredentialsInput{AuthType: "SNMP_V2C", Payload: map[string]any{}}
		if _, ok := validate(payload)["credentials.payload.community"]; !ok {
			t.Error("missing community error")
		}
	})
	t.Run("SNMP_V2C with community passes", func(t *testing.T) {
		payload := basePayload()
		payload.Credentials = &CredentialsInput{AuthType: "SNMP_V2C", Payload: map[string]any{"community": "public"}}
		if errs := validate(payload); len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
	})
	t.Run("SNMP_V3 requires auth+priv pair", func(t *testing.T) {
		payload := basePayload()
		payload.Credentials = &CredentialsInput{AuthType: "SNMP_V3", Payload: map[string]any{"username": "mon"}}
		errs := validate(payload)
		for _, key := range []string{
			"credentials.payload.auth_protocol",
			"credentials.payload.auth_password",
			"credentials.payload.priv_protocol",
			"credentials.payload.priv_password",
		} {
			if _, ok := errs[key]; !ok {
				t.Errorf("missing %s error", key)
			}
		}
	})
	t.Run("HTTP_BEARER requires token", func(t *testing.T) {
		payload := basePayload()
		payload.Credentials = &CredentialsInput{AuthType: "HTTP_BEARER", Payload: map[string]any{}}
		if _, ok := validate(payload)["credentials.payload.bearer_token"]; !ok {
			t.Error("missing bearer_token error")
		}
	})
	t.Run("S3_KEY requires both keys", func(t *testing.T) {
		payload := basePayload()
		payload.Credentials = &CredentialsInput{AuthType: "S3_KEY", Payload: map[string]any{"access_key": "AKIA"}}
		if _, ok := validate(payload)["credentials.payload.secret_key"]; !ok {
			t.Error("missing secret_key error")
		}
	})

	// Beszel XOR: token only or email+password, never both/neither.
	beszel := func(payload map[string]any) map[string][]string {
		p := basePayload()
		p.Credentials = &CredentialsInput{AuthType: "BESZEL", Payload: payload}

		return validate(p)
	}
	if _, ok := beszel(map[string]any{})["credentials.payload"]; !ok {
		t.Error("empty Beszel credentials accepted")
	}
	if _, ok := beszel(map[string]any{
		"api_token": "t", "email": "a@b.c", "password": "x",
	})["credentials.payload"]; !ok {
		t.Error("Beszel token+login together accepted")
	}
	if errs := beszel(map[string]any{"api_token": "t"}); len(errs) > 0 {
		t.Errorf("Beszel token-only rejected: %v", errs)
	}
	if errs := beszel(map[string]any{"email": "a@b.c", "password": "x"}); len(errs) > 0 {
		t.Errorf("Beszel login-only rejected: %v", errs)
	}

	// MAIL_AUTH needs a complete SMTP or IMAP pair.
	mail := func(payload map[string]any) map[string][]string {
		p := basePayload()
		p.Credentials = &CredentialsInput{AuthType: "MAIL_AUTH", Payload: payload}

		return validate(p)
	}
	if _, ok := mail(map[string]any{"smtp_username": "u"})["credentials.payload"]; !ok {
		t.Error("half SMTP pair accepted")
	}
	if errs := mail(map[string]any{"smtp_username": "u", "smtp_password": "p"}); len(errs) > 0 {
		t.Errorf("complete SMTP pair rejected: %v", errs)
	}

	// Unknown auth type is rejected like Rule::in.
	p := basePayload()
	p.Credentials = &CredentialsInput{AuthType: "OAUTH", Payload: map[string]any{}}
	if _, ok := validate(p)["credentials.auth_type"]; !ok {
		t.Error("unknown auth_type accepted")
	}
}

func TestValidateBeszelHubURLScheme(t *testing.T) {
	payload := basePayload()
	payload.TargetType = "BESZEL_SERVER"
	payload.ProtocolType = "BESZEL_API"
	payload.DashboardGroup = "MAIL_S3_BESZEL"
	payload.ConfigPayload = map[string]any{"hub_url": "beszel.local:8090"}

	errs := validate(payload)
	if _, ok := errs["config_payload.hub_url"]; !ok {
		t.Errorf("expected hub_url scheme error, got %v", errs)
	}

	payload.ConfigPayload["hub_url"] = "https://beszel.local:8090"
	if errs := validate(payload); len(errs) > 0 {
		t.Errorf("valid https hub rejected: %v", errs)
	}
}

func TestNormalizeForcesInternetToPing(t *testing.T) {
	payload := basePayload()
	payload.TargetType = "INTERNET"
	payload.ProtocolType = "HTTP_HEALTH"
	payload.ConfigPayload = map[string]any{"method": "GET"}

	normalize(&payload)

	if payload.ProtocolType != "ICMP_PING" {
		t.Errorf("protocol = %q, want ICMP_PING", payload.ProtocolType)
	}
	if icmpOnly, _ := payload.ConfigPayload["icmp_only"].(bool); !icmpOnly {
		t.Error("icmp_only not pinned")
	}
}

func TestNormalizeStripsIcmpOnlyAndMapsBeszelKey(t *testing.T) {
	payload := basePayload()
	payload.TargetType = "BESZEL_SERVER"
	payload.ConfigPayload = map[string]any{
		"icmp_only":        true,
		"beszel_system_id": "abc123",
	}

	normalize(&payload)

	if _, ok := payload.ConfigPayload["icmp_only"]; ok {
		t.Error("icmp_only survived on a non-INTERNET target")
	}
	if payload.ConfigPayload["beszel_system_name"] != "abc123" {
		t.Errorf("beszel_system_name = %v", payload.ConfigPayload["beszel_system_name"])
	}
	if _, ok := payload.ConfigPayload["beszel_system_id"]; ok {
		t.Error("beszel_system_id survived")
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike("50%_off"); !strings.Contains(got, `\%`) || !strings.Contains(got, `\_`) {
		t.Errorf("escapeLike(50%%_off) = %q", got)
	}
}

// ptrTo keeps the mutate-closure table readable; the production intPtr
// helper converts sql.NullInt32 scan results instead.
func ptrTo(value int) *int { return &value }
