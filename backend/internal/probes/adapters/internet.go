package adapters

import (
	"opslira/backend/internal/probes"
)

// Internet probes INTERNET targets: ICMP ping only — no TCP fallback, no SNMP
// reading. This is NetworkSnmpProbeAdapter with icmp_only=true, which the
// target controller stores server-side for every INTERNET target
// (InternetTargetTest); the factory maps ICMP_PING here.
type Internet struct{ Network }

// NewInternet builds the ping-only ICMP_PING adapter.
func NewInternet() *Internet {
	return &Internet{Network: Network{adapterBase: newBase(), forceICMPOnly: true}}
}

func (a *Internet) SupportsProtocol(protocolType string) bool {
	return protocolType == probes.ProtocolICMPPing
}

func (a *Internet) Configure(config, credentials map[string]any) probes.Adapter {
	a.apply(config, credentials)
	return a
}

func (a *Internet) Tune(timeoutSeconds, degradedThresholdMs int) probes.Adapter {
	a.tune(timeoutSeconds, degradedThresholdMs)
	return a
}
