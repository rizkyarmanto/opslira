package adapters

import (
	"errors"
	"testing"

	"opslira/backend/internal/probes"
)

// Every protocol_type of mt_protocol_type_check must resolve to an adapter
// that claims it, exactly like the PHP ProbeAdapterFactory map.
func TestFactorySupportsAllProtocols(t *testing.T) {
	factory := NewFactory(Deps{})

	want := []string{
		probes.ProtocolICMPSNMP,
		probes.ProtocolICMPPing,
		probes.ProtocolHTTPHealth,
		probes.ProtocolWHOISSSL,
		probes.ProtocolDBNativePing,
		probes.ProtocolSMTPDNSBL,
		probes.ProtocolS3Head,
		probes.ProtocolBeszelAPI,
	}
	if got := len(factory.SupportedProtocols()); got != len(want) {
		t.Fatalf("SupportedProtocols() = %d entries, want %d", got, len(want))
	}

	for _, protocol := range want {
		adapter, err := factory.Make(protocol)
		if err != nil {
			t.Fatalf("Make(%s) error: %v", protocol, err)
		}
		if !adapter.SupportsProtocol(protocol) {
			t.Errorf("adapter for %s does not support it", protocol)
		}
	}
}

func TestFactoryUnknownProtocol(t *testing.T) {
	factory := NewFactory(Deps{})

	adapter, err := factory.Make("CARRIER_PIGEON")
	if !errors.Is(err, probes.ErrUnknownProtocol) {
		t.Fatalf("Make error = %v, want ErrUnknownProtocol", err)
	}
	if adapter != nil {
		t.Error("adapter must be nil for unknown protocol")
	}
	if err.Error() != "unknown protocol_type: CARRIER_PIGEON" {
		t.Errorf("error message = %q", err.Error())
	}
}
