package incidents

import (
	"strings"
	"testing"

	"opslira/backend/internal/config"
	"opslira/backend/internal/model"
)

func testConfig() config.Config {
	return config.Config{Timezone: "Asia/Jakarta"}
}

// buildSubject must mirror AlertNotificationService::buildSubject prefixes so
// operators can triage from the subject line alone.
func TestBuildSubject(t *testing.T) {
	svc := &Service{}
	target := model.Target{Name: "core-switch-01"}

	cases := []struct {
		event string
		want  string
	}{
		{"DOWN", "[CRITICAL] DOWN: core-switch-01"},
		{"RECOVERED", "[RESOLVED] RECOVERED: core-switch-01"},
		{"SOMETHING_ELSE", "[ALERT]: core-switch-01"},
	}
	for _, tc := range cases {
		if got := svc.buildSubject(target, tc.event); got != tc.want {
			t.Errorf("buildSubject(%q) = %q, want %q", tc.event, got, tc.want)
		}
	}
}

// buildBody must carry every field the Laravel alert email included, with the
// endpoint joined as host:port and the WIB timestamp suffix. RECOVERED is used
// because the DOWN branch queries the DB for the incident root cause.
func TestBuildBody(t *testing.T) {
	svc := &Service{cfg: testConfig()}
	port := 443
	target := model.Target{
		Name:           "api-prod-01",
		HostOrEndpoint: "api.example.com",
		Port:           &port,
		TargetType:     "WEB_APP",
		ProtocolType:   "HTTP_HEALTH",
		Environment:    "PRODUCTION",
	}

	body := svc.buildBody(t.Context(), target, "RECOVERED")

	for _, want := range []string{
		"🟢 Status change: RECOVERED",
		"Target: api-prod-01",
		"Endpoint: api.example.com:443",
		"Type: WEB_APP (HTTP_HEALTH)",
		"Environment: PRODUCTION",
		"WIB",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}
