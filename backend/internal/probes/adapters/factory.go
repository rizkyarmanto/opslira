package adapters

import (
	"fmt"
	"sort"
	"time"

	"opslira/backend/internal/probes"
)

// Deps are the factory's shared optional dependencies.
type Deps struct {
	// Cache backs the Beszel token cache and the WHOIS rate limit (Redis DB1
	// through probes.NewRedisCache — the go-redis equivalent of Laravel's
	// Cache facade). Nil disables caching in those two adapters.
	Cache probes.Cache
	// BeszelTokenCacheTTL is BESZEL_TOKEN_CACHE_TTL (default 43200 s).
	BeszelTokenCacheTTL time.Duration
}

// Factory mirrors ProbeAdapterFactory: one protocol_type -> adapter mapping,
// the same table as the PHP $map. Every Make returns a fresh adapter because
// adapters are stateful.
type Factory struct{ deps Deps }

// NewFactory builds the factory; deps.Cache may be nil.
func NewFactory(deps Deps) *Factory { return &Factory{deps: deps} }

var protocolMap = map[string]func(Deps) probes.Adapter{
	probes.ProtocolICMPSNMP:     func(Deps) probes.Adapter { return NewNetwork() },
	probes.ProtocolICMPPing:     func(Deps) probes.Adapter { return NewInternet() },
	probes.ProtocolHTTPHealth:   func(Deps) probes.Adapter { return NewHTTPHealth() },
	probes.ProtocolWHOISSSL:     func(d Deps) probes.Adapter { return NewDomainSSL(d.Cache) },
	probes.ProtocolDBNativePing: func(Deps) probes.Adapter { return NewDatabasePing() },
	probes.ProtocolSMTPDNSBL:    func(Deps) probes.Adapter { return NewMailServer() },
	probes.ProtocolS3Head:       func(Deps) probes.Adapter { return NewS3Head() },
	probes.ProtocolBeszelAPI:    func(d Deps) probes.Adapter { return NewBeszelAPI(d.Cache, d.BeszelTokenCacheTTL) },
}

// Make resolves a protocol_type; unknown values return ErrUnknownProtocol,
// which the HTTP layer maps to 422 {"message":"unknown protocol_type"}.
func (f *Factory) Make(protocolType string) (probes.Adapter, error) {
	build, ok := protocolMap[protocolType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", probes.ErrUnknownProtocol, protocolType)
	}
	return build(f.deps), nil
}

// SupportedProtocols lists the enum keys, sorted for stable output.
func (f *Factory) SupportedProtocols() []string {
	protocols := make([]string, 0, len(protocolMap))
	for protocol := range protocolMap {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)
	return protocols
}
