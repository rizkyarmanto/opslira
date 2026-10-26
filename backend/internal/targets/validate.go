package targets

import (
	"strconv"
	"strings"

	"opslira/backend/internal/httpx"
)

// Enum sets mirrored from StoreTargetRequest rules and the CHECK constraints
// (target_type/protocol_type include the INTERNET additions from
// 2026_10_20_000001; auth types follow 2026_10_19_000001).
var (
	dashboardGroups = []string{"NETWORK", "APP_DOMAIN_DB", "MAIL_S3_BESZEL"}
	targetTypes     = []string{"NETWORK_DEVICE", "WEB_APP", "DOMAIN", "DATABASE", "MAIL_SERVER", "S3_STORAGE", "BESZEL_SERVER", "INTERNET"}
	environments    = []string{"PRODUCTION", "STAGING", "DEVELOPMENT", "INTERNAL"}
	protocolTypes   = []string{"ICMP_SNMP", "HTTP_HEALTH", "WHOIS_SSL", "DB_NATIVE_PING", "SMTP_DNSBL", "S3_HEAD", "BESZEL_API", "ICMP_PING"}
	authTypes       = []string{"NONE", "DB_AUTH", "SNMP_V2C", "SNMP_V3", "S3_KEY", "BESZEL", "HTTP_BASIC", "HTTP_BEARER", "MAIL_AUTH"}
)

// CredentialsInput is the `credentials` object of a target payload; nil means
// the key was absent (update leaves stored credentials untouched).
type CredentialsInput struct {
	AuthType string         `json:"auth_type"`
	Payload  map[string]any `json:"payload"`
}

// TargetPayload is the store/update request body.
type TargetPayload struct {
	Name                 string            `json:"name"`
	DashboardGroup       string            `json:"dashboard_group"`
	TargetType           string            `json:"target_type"`
	Environment          string            `json:"environment"`
	HostOrEndpoint       string            `json:"host_or_endpoint"`
	Port                 *int              `json:"port"`
	ProtocolType         string            `json:"protocol_type"`
	ConfigPayload        map[string]any    `json:"config_payload"`
	CheckIntervalSeconds *int              `json:"check_interval_seconds"`
	TimeoutSeconds       *int              `json:"timeout_seconds"`
	RetryThreshold       *int              `json:"retry_threshold"`
	DegradedThresholdMs  *int              `json:"degraded_threshold_ms"`
	Credentials          *CredentialsInput `json:"credentials"`
}

// validate mirrors StoreTargetRequest::rules + withValidator for one payload
// (UpdateTargetRequest extends StoreTargetRequest unchanged). Message text
// follows vendor/laravel/framework lang/en/validation.php verbatim, with
// attributes humanized the way Laravel renders them (underscores to spaces,
// dots kept).
func validate(p TargetPayload) httpx.ValidationErrors {
	errs := httpx.ValidationErrors{}

	if strings.TrimSpace(p.Name) == "" {
		add(errs, "name", "The name field is required.")
	} else if len(p.Name) > 150 {
		add(errs, "name", "The name field must not be greater than 150 characters.")
	}

	if !contains(dashboardGroups, p.DashboardGroup) {
		add(errs, "dashboard_group", "The selected dashboard group is invalid.")
	}
	if !contains(targetTypes, p.TargetType) {
		add(errs, "target_type", "The selected target type is invalid.")
	}
	if !contains(environments, p.Environment) {
		add(errs, "environment", "The selected environment is invalid.")
	}

	if strings.TrimSpace(p.HostOrEndpoint) == "" {
		add(errs, "host_or_endpoint", "The host or endpoint field is required.")
	} else if len(p.HostOrEndpoint) > 255 {
		add(errs, "host_or_endpoint", "The host or endpoint field must not be greater than 255 characters.")
	}

	if p.Port != nil && (*p.Port < 1 || *p.Port > 65535) {
		add(errs, "port", "The port field must be between 1 and 65535.")
	}

	if !contains(protocolTypes, p.ProtocolType) {
		add(errs, "protocol_type", "The selected protocol type is invalid.")
	}

	if p.CheckIntervalSeconds != nil && *p.CheckIntervalSeconds < 10 {
		add(errs, "check_interval_seconds", "The check interval seconds field must be at least 10.")
	}
	if p.TimeoutSeconds != nil && (*p.TimeoutSeconds < 1 || *p.TimeoutSeconds > 60) {
		add(errs, "timeout_seconds", "The timeout seconds field must be between 1 and 60.")
	}
	if p.RetryThreshold != nil && (*p.RetryThreshold < 1 || *p.RetryThreshold > 10) {
		add(errs, "retry_threshold", "The retry threshold field must be between 1 and 10.")
	}
	if p.DegradedThresholdMs != nil && *p.DegradedThresholdMs < 1 {
		add(errs, "degraded_threshold_ms", "The degraded threshold ms field must be at least 1.")
	}

	// Rule order matches StoreTargetRequest; the after() callbacks follow the
	// rules in PHP order: Beszel XOR, hub scheme, MAIL_AUTH.
	if p.Credentials != nil {
		validateCredentials(p.Credentials, errs)
		validateBeszelXOR(p.Credentials, errs)
	}
	validateConfig(p, errs)
	if p.Credentials != nil {
		validateMailPair(p.Credentials, errs)
	}

	return errs
}

func validateConfig(p TargetPayload, errs httpx.ValidationErrors) {
	if p.TargetType != "BESZEL_SERVER" || p.ConfigPayload == nil {
		return
	}
	hub, _ := p.ConfigPayload["hub_url"].(string)
	if hub == "" {
		hub, _ = p.ConfigPayload["host"].(string)
	}
	if hub != "" && !strings.HasPrefix(hub, "http://") && !strings.HasPrefix(hub, "https://") {
		add(errs, "config_payload.hub_url", "Beszel hub URL must start with http:// or https://.")
	}
}

// validateCredentials covers credentials.* rules only; the XOR/MAIL_AUTH
// after() checks live in validateCredentialPairs so the bag order follows
// PHP's rules-then-after() sequence.
func validateCredentials(c *CredentialsInput, errs httpx.ValidationErrors) {
	authType := c.AuthType
	if authType == "" {
		return // nullable rule: no auth type means no credential row
	}
	if !contains(authTypes, authType) {
		add(errs, "credentials.auth_type", "The selected credentials.auth type is invalid.")
		return
	}

	payload := c.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	requireString(payload, "credentials.payload.community", 255, authType == "SNMP_V2C", errs)
	requireString(payload, "credentials.payload.username", 255,
		authType == "HTTP_BASIC" || authType == "DB_AUTH" || authType == "SNMP_V3", errs)
	requireString(payload, "credentials.payload.password", 255, authType == "HTTP_BASIC" || authType == "DB_AUTH", errs)

	if authType == "SNMP_V3" {
		requireEnum(payload, "credentials.payload.auth_protocol", []string{"MD5", "SHA"}, errs)
		requireString(payload, "credentials.payload.auth_password", 0, true, errs)
		requireEnum(payload, "credentials.payload.priv_protocol", []string{"DES", "AES"}, errs)
		requireString(payload, "credentials.payload.priv_password", 0, true, errs)
	}

	requireString(payload, "credentials.payload.bearer_token", 2048, authType == "HTTP_BEARER", errs)
	requireString(payload, "credentials.payload.access_key", 255, authType == "S3_KEY", errs)
	requireString(payload, "credentials.payload.secret_key", 512, authType == "S3_KEY", errs)

	// Always-nullable pairs (present must still be a string of legal length).
	requireString(payload, "credentials.payload.smtp_username", 255, false, errs)
	requireString(payload, "credentials.payload.smtp_password", 255, false, errs)
	requireString(payload, "credentials.payload.imap_username", 255, false, errs)
	requireString(payload, "credentials.payload.imap_password", 255, false, errs)
}

// validateBeszelXOR ports withValidator's first after() callback: BESZEL
// credentials are XOR — an API token, or both email and password.
func validateBeszelXOR(c *CredentialsInput, errs httpx.ValidationErrors) {
	if c.AuthType != "BESZEL" {
		return
	}
	payload := c.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	hasToken := nonEmptyString(payload, "api_token")
	hasLogin := nonEmptyString(payload, "email") && nonEmptyString(payload, "password")
	if hasToken == hasLogin {
		add(errs, "credentials.payload", "Beszel credentials require either an API token or both email and password.")
	}
}

// validateMailPair ports the third after() callback: one row per target, so
// the login choice decides which pair is present.
func validateMailPair(c *CredentialsInput, errs httpx.ValidationErrors) {
	if c.AuthType != "MAIL_AUTH" {
		return
	}
	payload := c.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	hasSMTP := nonEmptyString(payload, "smtp_username") && nonEmptyString(payload, "smtp_password")
	hasIMAP := nonEmptyString(payload, "imap_username") && nonEmptyString(payload, "imap_password")
	if !hasSMTP && !hasIMAP {
		add(errs, "credentials.payload", "SMTP/IMAP login requires both an SMTP username and password, or both an IMAP username and password.")
	}
}

// requireString enforces presence/type/length for one credential payload key.
// required toggles the requiredIf rule; an explicit JSON null counts as
// absent (nullable), except that a required key still fails.
func requireString(payload map[string]any, key string, maxLen int, required bool, errs httpx.ValidationErrors) {
	short := key[strings.LastIndex(key, ".")+1:]
	label := humanizeAttr(key)
	value, present := payload[short]
	if present && value == nil {
		present = false
	}

	if !present {
		if required {
			add(errs, key, "The "+label+" field is required.")
		}
		return
	}
	s, ok := value.(string)
	if !ok {
		add(errs, key, "The "+label+" field must be a string.")
		return
	}
	if required && s == "" {
		add(errs, key, "The "+label+" field is required.")
		return
	}
	if maxLen > 0 && len(s) > maxLen {
		add(errs, key, "The "+label+" field must not be greater than "+strconv.Itoa(maxLen)+" characters.")
	}
}

// requireEnum enforces presence + membership for a credential payload key
// (auth_protocol / priv_protocol on SNMP_V3).
func requireEnum(payload map[string]any, key string, allowed []string, errs httpx.ValidationErrors) {
	short := key[strings.LastIndex(key, ".")+1:]
	label := humanizeAttr(key)
	value, present := payload[short]
	if !present || value == nil {
		add(errs, key, "The "+label+" field is required.")
		return
	}
	s, ok := value.(string)
	if !ok || !contains(allowed, s) {
		add(errs, key, "The selected "+label+" is invalid.")
	}
}

func nonEmptyString(payload map[string]any, key string) bool {
	value, present := payload[key]
	if !present {
		return false
	}
	s, ok := value.(string)

	return ok && strings.TrimSpace(s) != ""
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}

// humanizeAttr renders a validation attribute the way Laravel does:
// underscores become spaces, dots stay (getDisplayableAttribute).
func humanizeAttr(key string) string {
	return strings.ReplaceAll(key, "_", " ")
}

// add appends a validation message under a possibly dotted field path.
func add(errs httpx.ValidationErrors, field, message string) {
	errs[field] = append(errs[field], message)
}
