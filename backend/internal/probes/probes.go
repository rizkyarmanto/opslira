// Package probes ports the PHP probe pipeline (app/Modules/Probes): a
// protocol_type resolves to an Adapter through a Factory, the adapter is
// configured with the target's config_payload plus decrypted credentials and
// executed, and the ProbeResult is written to monitoring_logs + Redis state by
// the ExecutionService. Adapters live in the adapters subpackage.
//
// There is no equivalent of inertia/predis/Eloquent here: adapters talk to the
// network directly, persistence is raw SQL through pgx, state is Redis.
package probes

import (
	"context"
	"errors"
	"time"

	"opslira/backend/internal/model"
)

// ErrUnknownProtocol is returned by Factory.Make for a protocol_type outside
// the enum; the HTTP layer maps it to 422 {"message":"unknown protocol_type"}.
var ErrUnknownProtocol = errors.New("unknown protocol_type")

// protocol_type values, mirroring mt_protocol_type_check.
const (
	ProtocolICMPSNMP     = "ICMP_SNMP"
	ProtocolHTTPHealth   = "HTTP_HEALTH"
	ProtocolWHOISSSL     = "WHOIS_SSL"
	ProtocolDBNativePing = "DB_NATIVE_PING"
	ProtocolSMTPDNSBL    = "SMTP_DNSBL"
	ProtocolS3Head       = "S3_HEAD"
	ProtocolBeszelAPI    = "BESZEL_API"
	ProtocolICMPPing     = "ICMP_PING"
)

// Events passed to StateChangeHook, matching the $event values of
// ProcessStateChangeAlertJob.
const (
	EventDown      = "DOWN"
	EventRecovered = "RECOVERED"
)

// Adapter mirrors App\Modules\Probes\Contracts\ProbeAdapterInterface: a
// Strategy implementation configured from a target's config_payload +
// decrypted credentials, then executed once. Failures are DOWN results, not
// Go errors.
type Adapter interface {
	// Configure supplies config_payload (with host/port injected by the
	// execution service) and the decrypted credential map, which may be empty.
	// Returns the adapter for chaining, like PHP configure(): static.
	Configure(config, credentials map[string]any) Adapter
	// Execute runs one probe against the network.
	Execute(ctx context.Context) ProbeResult
	// SupportsProtocol reports whether this adapter handles the protocol_type.
	SupportsProtocol(protocolType string) bool
}

// TunedAdapter is BaseProbeAdapter::forTarget()/withTimeoutSeconds(): the
// pre-save test-connection request has no target row, so the timeout must be
// set explicitly instead of inheriting the 5 s default.
type TunedAdapter interface {
	Adapter
	// Tune applies the per-target knobs. Values <= 0 keep the defaults
	// (5 s timeout, 1000 ms degraded threshold).
	Tune(timeoutSeconds, degradedThresholdMs int) Adapter
}

// Factory resolves a protocol_type to a fresh (stateful) Adapter.
type Factory interface {
	// Make returns ErrUnknownProtocol for values outside the enum.
	Make(protocolType string) (Adapter, error)
	SupportedProtocols() []string
}

// Credential mirrors the joined target_credentials row (three base64
// columns); nil means the target has no credentials.
type Credential struct {
	EncryptedPayload string
	EncryptionIV     string
	EncryptionTag    string
}

// CredentialDecryptor is satisfied by *credentials.Service (AES-256-GCM,
// byte-compatible with CredentialEncryptionService.php).
type CredentialDecryptor interface {
	Decrypt(payload, iv, tag string) (map[string]any, error)
}

// Cache is the Laravel Cache facade equivalent backed by Redis DB1. It backs
// the Beszel token cache and the WHOIS rate limit; nil disables caching.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, value string, ttl time.Duration)
	Del(ctx context.Context, keys ...string)
}

// StateChangeHook is notified inline (QUEUE_CONNECTION=sync) after a state
// change, like ProcessStateChangeAlertJob::dispatchSync. incidentID is only
// set for DOWN events; RECOVERED events carry "".
type StateChangeHook func(ctx context.Context, target model.Target, event string, incidentID string)
